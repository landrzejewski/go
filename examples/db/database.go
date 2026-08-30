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
	"syscall"
	"training.pl/go/common"
)

const stateFileSuffix = ".state"

// Typowane stałe zamiast gołych stringów: literówka w nazwie akcji jest teraz
// błędem kompilacji, a nie cichym zgubieniem komendy (wywołujący czekałby
// wtedy wiecznie na <-reply).
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
	idGenerator IdGenerator
	// done jest zamykane przez run() po opróżnieniu kanału komend - dzięki
	// temu Close() wie, kiedy nikt już nie pisze do pliku ani do state.
	done chan struct{}
}

type DatabaseState struct {
	Records map[int64]*Record
	LastId  int64
}

func Db(filepath string, idGenerator IdGenerator) *Database {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0644)
	catchFatal(err, "Failed to open database")
	var state DatabaseState
	bytes, err := os.ReadFile(filepath + stateFileSuffix)
	if err != nil {
		state = DatabaseState{Records: make(map[int64]*Record), LastId: 0}
	} else {
		catchFatal(common.FromBytes(bytes, &state), "Failed reading database state")
	}
	// Zasianie generatora zapisanym stanem. Bez tego LastId było polem
	// martwym - zapisywanym na dysk, ale nigdy nieczytanym - a Sequence
	// startowało od zera przy każdym uruchomieniu. Po restarcie id zaczynały
	// się od 1 i albo trafiały na "record with id 1 already exists", albo po
	// cichu nadpisywały istniejący rekord.
	if seedable, ok := idGenerator.(interface{ seed(int64) }); ok {
		seedable.seed(state.LastId)
	}

	return &Database{
		file:        file,
		commands:    make(chan command, 100),
		state:       &state,
		idGenerator: idGenerator,
		done:        make(chan struct{}),
	}
}

//func catchFatal(err error, description func() string) {
//	if err != nil {
//		log.Fatal(description())
//	}
//}

func catchFatal(err error, description string) {
	if err != nil {
		log.Fatal(description + ": " + err.Error())
	}
}

func (d *Database) Close() {
	// close(d.commands) samo w sobie NIE czeka na run() - bez <-d.done goroutine
	// run mogłaby jeszcze pisać do pliku i mutować state, podczas gdy Close
	// zamyka plik i serializuje ten sam state (wyścig danych + zapis do
	// zamkniętego deskryptora). Komendy z bufora też zostałyby porzucone.
	close(d.commands)
	<-d.done

	// Kolejność: najpierw zapis stanu (używa d.file.Name()), potem zamknięcie pliku.
	catchFatal(d.saveState(), "Save database state failed")
	catchFatal(d.file.Close(), "Close database file failed")
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
			// Bez tej gałęzi nieznana akcja nie dawała odpowiedzi, a wywołujący
			// blokował się na <-reply na zawsze.
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
	_, exit := d.state.Records[id]
	if exit {
		return &Result{nil, fmt.Errorf("record with id %d already exists", id)}
	}
	length, err := d.file.WriteAt(bytes, offset)
	if err != nil {
		return &Result{Record: nil, Error: err}
	}
	record := &Record{id, offset, int64(length)}
	d.state.Records[id] = record
	d.state.LastId = id // utrwalane niżej przez saveState - patrz Db()
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
	return &Result{record, err}
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
	// Bez zapisu stanu (co robią już create i delete) indeks na dysku wskazywał
	// stary offset/length, podczas gdy nowe bajty leżały na końcu pliku -
	// każda awaria gubiła aktualizację i zostawiała indeks wskazujący śmieci.
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
	// go db.run() PRZED defer db.Close(): defery wykonują się w odwrotnej
	// kolejności rejestracji, a Close czeka teraz na run.
	go db.run()
	defer db.Close()

	user := User{"Jan", "Kowalski", 25, true}
	result := db.Create(&user)
	// Bez sprawdzenia błędu kolejna linia dereferencjonowałaby nil Record.
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

	// UWAGA: router.Run blokuje aż do zakończenia procesu, więc `defer db.Close()`
	// nigdy by się nie wykonał - a wraz z nim zapis stanu bazy. Dlatego
	// zamykamy bazę jawnie po wyjściu z Run i przechwytujemy sygnał
	// przerwania, żeby Ctrl-C też utrwalił stan.
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
		log.Println("Serwer zakończył działanie:", err)
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
