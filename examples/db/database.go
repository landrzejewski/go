package db

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"training.pl/go/examples/common"
)

const stateFileSuffix = ".state"

// Typed constants instead of bare strings: a typo in an action name is now a
// compile error rather than a silently dropped command (the caller would then
// wait on <-reply forever).
type action string

const (
	actionInsert action = "insert"
	actionFind   action = "find"
	actionUpdate action = "update"
	actionDelete action = "delete"
)

type command struct {
	action action
	id     int64
	input  any
	output any
	reply  chan *Result
}

type Result struct {
	Record *Record
	Error  error
}

type Record struct {
	Id     int64
	Offset int64
	Length int64
}

type Database struct {
	file        *os.File
	commands    chan command
	state       *DatabaseState
	idGenerator IDGenerator
	// done is closed by run() once the command channel has been drained - that is
	// how Close() knows nobody is still writing to the file or to state.
	done chan struct{}
	// closeOnce keeps Close idempotent: DatabaseExercise calls it both from the
	// signal goroutine and from a defer, and close() on an already closed channel
	// panics.
	closeOnce sync.Once
}

type DatabaseState struct {
	Records map[int64]*Record
	LastId  int64
}

// NOTE ON A KNOWN LIMITATION: space is never reclaimed. delete removes only the
// index entry, and update always appends at endOffset(), so the file grows without
// bound. Exercise 8 in notes.md asks for reuse of freed space - implementing it
// means keeping a free list of (offset, length) holes and picking one that fits,
// plus compaction when fragmentation gets bad.

func Db(filepath string, idGenerator IDGenerator) *Database {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0644)
	catchFatal(err, "Failed to open database")
	var state DatabaseState
	bytes, err := os.ReadFile(filepath + stateFileSuffix)
	if err != nil {
		state = DatabaseState{Records: make(map[int64]*Record), LastId: 0}
	} else {
		catchFatal(common.FromBytes(bytes, &state), "Failed reading database state")
	}
	// Seed the generator from the stored state. Without this LastId was a dead
	// field - written to disk but never read back - and Sequence restarted from
	// zero on every run. After a restart ids began at 1 again and either hit
	// "record with id 1 already exists" or silently overwrote an existing record.
	idGenerator.seed(state.LastId)

	return &Database{
		file:        file,
		commands:    make(chan command, 100),
		state:       &state,
		idGenerator: idGenerator,
		done:        make(chan struct{}),
	}
}

// catchFatal calls log.Fatal, which is acceptable only because this package is
// demo code with its own entry points. A real library returns errors instead of
// terminating its caller's process.
func catchFatal(err error, description string) {
	if err != nil {
		log.Fatal(description + ": " + err.Error())
	}
}

// Close is safe to call more than once.
func (d *Database) Close() {
	d.closeOnce.Do(func() {
		// close(d.commands) on its own does NOT wait for run() - without <-d.done
		// the run goroutine could still be writing to the file and mutating state
		// while Close closes the file and serialises that same state (a data race
		// plus a write to a closed descriptor). Buffered commands would be dropped
		// too.
		close(d.commands)
		<-d.done

		// Order matters: save the state first (it uses d.file.Name()), then close
		// the file.
		catchFatal(d.saveState(), "Save database state failed")
		catchFatal(d.file.Close(), "Close database file failed")
	})
}

func (d *Database) saveState() error {
	bytes, err := common.ToBytes(d.state)
	if err != nil {
		return err
	}
	return os.WriteFile(d.file.Name()+stateFileSuffix, bytes, 0644)
}

func (d *Database) run() {
	defer close(d.done)
	for cmd := range d.commands {
		switch cmd.action {
		case actionInsert:
			cmd.reply <- d.create(cmd.input)
		case actionFind:
			cmd.reply <- d.read(cmd.id, cmd.output)
		case actionUpdate:
			cmd.reply <- d.update(cmd.id, cmd.input)
		case actionDelete:
			cmd.reply <- d.delete(cmd.id)
		default:
			// Without this branch an unknown action produced no reply and the
			// caller blocked on <-reply forever.
			cmd.reply <- &Result{nil, fmt.Errorf("unknown action %q", cmd.action)}
		}
	}
}

func (d *Database) create(object any) *Result {
	bytes, err := common.ToBytes(object)
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	offset, err := d.endOffset()
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	id := d.idGenerator.next()
	_, exists := d.state.Records[id]
	if exists {
		return &Result{nil, fmt.Errorf("record with id %d already exists", id)}
	}
	length, err := d.file.WriteAt(bytes, offset)
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	record := &Record{id, offset, int64(length)}
	d.state.Records[id] = record
	d.state.LastId = id // persisted by saveState below - see Db()
	if err := d.saveState(); err != nil {
		return &Result{Record: nil, Error: err}
	}
	return &Result{record, nil}
}

func (d *Database) read(id int64, object any) *Result {
	record, exists := d.state.Records[id]
	if !exists {
		return &Result{nil, fmt.Errorf("record with id %d not found", id)}
	}
	bytes := make([]byte, record.Length)
	_, err := d.file.ReadAt(bytes, record.Offset)
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	err = common.FromBytes(bytes, object)
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	return &Result{record, nil}
}

func (d *Database) delete(id int64) *Result {
	_, exists := d.state.Records[id]
	if !exists {
		return &Result{nil, fmt.Errorf("record with id %d not found", id)}
	}
	delete(d.state.Records, id)
	if err := d.saveState(); err != nil {
		return &Result{nil, err}
	}
	return &Result{nil, nil}
}

func (d *Database) update(id int64, object any) *Result {
	bytes, err := common.ToBytes(object)
	if err != nil {
		return &Result{nil, err}
	}
	record, exists := d.state.Records[id]
	if !exists {
		return &Result{nil, fmt.Errorf("record with id %d not found", id)}
	}
	offset, err := d.endOffset()
	if err != nil {
		return &Result{nil, err}
	}
	length, err := d.file.WriteAt(bytes, offset)
	if err != nil {
		return &Result{nil, err}
	}
	record.Offset = offset
	record.Length = int64(length)
	// Without saving the state (which create and delete already do) the on-disk
	// index pointed at the old offset/length while the new bytes sat at the end of
	// the file - any crash lost the update and left the index pointing at garbage.
	if err := d.saveState(); err != nil {
		return &Result{nil, err}
	}
	return &Result{record, nil}
}

func (d *Database) endOffset() (int64, error) {
	return d.file.Seek(0, io.SeekEnd)
}

func (d *Database) Create(input any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: actionInsert, input: input, reply: reply}
	return <-reply
}

func (d *Database) Read(id int64, output any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: actionFind, id: id, output: output, reply: reply}
	return <-reply
}

func (d *Database) Delete(id int64) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: actionDelete, id: id, reply: reply}
	return <-reply
}

func (d *Database) Update(id int64, input any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: actionUpdate, id: id, input: input, reply: reply}
	return <-reply
}

func DatabaseTest() {
	db := Db("users.db", &Sequence{})
	// go db.run() BEFORE defer db.Close(): defers run in reverse registration
	// order, and Close now waits for run.
	go db.run()
	defer db.Close()

	user := User{"Jan", "Kowalski", 25, true}
	result := db.Create(&user)
	// Without checking the error the next line would dereference a nil Record.
	if result.Error != nil {
		log.Println("Create failed:", result.Error)
		return
	}
	fmt.Println(result.Record, result.Error)
	id := result.Record.Id

	user.IsActive = false
	result = db.Update(id, &user)
	fmt.Println(result.Record, result.Error)

	loadedUser := &User{}
	result = db.Read(id, loadedUser)
	fmt.Println(result.Record, result.Error, loadedUser)

	result = db.Delete(id)
	fmt.Println(result.Record, result.Error)
}

type User struct {
	FirstName string
	LastName  string
	Age       int16
	IsActive  bool
}

func DatabaseExercise() {
	db := Db("users.db", &Sequence{})
	go db.run()

	// NOTE: router.Run blocks until the process ends, so `defer db.Close()` would
	// never run - and the database state would never be written. Hence the explicit
	// close after Run returns, plus a signal handler so that Ctrl-C persists the
	// state too. Close is idempotent (sync.Once), so both paths firing is fine.
	defer db.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		db.Close()
		os.Exit(0)
	}()

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
	})

	router.POST("/users", createUser)
	router.GET("/users/:id", getUser)
	router.PUT("/users/:id", updateUser)
	router.DELETE("/users/:id", deleteUser)

	if err := router.Run(":8080"); err != nil {
		log.Println("server stopped:", err)
	}
}

func getDb(c *gin.Context) *Database {
	db, _ := c.Get("db")
	return db.(*Database)
}

type CreateUserResponse struct {
	Id int64
}

func createUser(c *gin.Context) {
	var user User
	err := c.Bind(&user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	result := getDb(c).Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	c.Header("Location", fmt.Sprintf("/api/users/%d", result.Record.Id))
	c.JSON(http.StatusCreated, &CreateUserResponse{result.Record.Id})
}

func getUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	user := User{}
	result := getDb(c).Read(id, &user)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	c.JSON(http.StatusOK, &user)
}

func updateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	var user User
	err = c.Bind(&user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	result := getDb(c).Update(id, &user)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	result := getDb(c).Delete(id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	c.Status(http.StatusNoContent)
}
