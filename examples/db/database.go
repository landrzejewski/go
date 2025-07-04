package db

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"training.pl/go/common"
)

const stateFileSuffix = ".state"

type command struct {
	action string
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
	return &Database{file, make(chan command, 100), &state, idGenerator}
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
	close(d.commands)
	// catchFatal(d.file.Close(), func() string { return "Close database file failed"})
	catchFatal(d.file.Close(), "Close database file failed")
	catchFatal(d.saveState(), "Save database state failed")
}

func (d *Database) saveState() error {
	bytes, err := common.ToBytes(d.state)
	if err != nil {
		return err
	}
	return os.WriteFile(d.file.Name()+stateFileSuffix, bytes, 0644)
}

func (d *Database) run() {
	for cmd := range d.commands {
		switch cmd.action {
		case "insert":
			cmd.reply <- d.create(cmd.input)
		case "find":
			cmd.reply <- d.read(cmd.id, cmd.output)
		case "update":
			cmd.reply <- d.update(cmd.id, cmd.input)
		case "delete":
			cmd.reply <- d.delete(cmd.id)
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
	return &Result{record, nil}
}

func (d *Database) endOffset() (int64, error) {
	return d.file.Seek(0, io.SeekEnd)
}

func (d *Database) Create(input any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: "insert", input: input, reply: reply}
	return <-reply
}

func (d *Database) Read(id int64, output any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: "find", id: id, output: output, reply: reply}
	return <-reply
}

func (d *Database) Delete(id int64) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: "delete", id: id, reply: reply}
	return <-reply
}

func (d *Database) Update(id int64, input any) *Result {
	reply := make(chan *Result)
	d.commands <- command{action: "update", id: id, input: input, reply: reply}
	return <-reply
}

func DatabaseTest() {
	db := Db("users.db", &Sequence{})
	defer db.Close()
	go db.run()

	user := User{"Jan", "Kowalski", 25, true}
	result := db.Create(&user)
	fmt.Println(result.Record, result.Error)

	user.IsActive = false
	result = db.Update(result.Record.Id, &user)
	fmt.Println(result.Record, result.Error)

	loadedUser := &User{}
	result = db.Read(result.Record.Id, loadedUser)
	fmt.Println(result.Record, result.Error, loadedUser)

	result = db.Delete(result.Record.Id)
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
	defer db.Close()
	go db.run()

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
	})

	router.POST("/users", createUser)
	router.GET("/users/:id", getUser)
	router.PUT("/users/:id", updateUser)
	router.DELETE("/users/:id", deleteUser)

	router.Run(":8080")
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
	result := getDb(c).Create(user)
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

/*const stateFileSuffix = ".state"

type Record struct {
	Id     int64
	Offset int64
	Length int64
}

type Database struct {
	file        *os.File
	lock        *sync.Mutex
	state       *DatabaseState
	idGenerator IdGenerator
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
	return &Database{file, &sync.Mutex{}, &state, idGenerator}
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
	// catchFatal(d.file.Close(), func() string { return "Close database file failed"})
	catchFatal(d.file.Close(), "Close database file failed")
	catchFatal(d.saveState(), "Save database state failed")
}

func (d *Database) saveState() error {
	bytes, err := common.ToBytes(d.state)
	if err != nil {
		return err
	}
	return os.WriteFile(d.file.Name()+stateFileSuffix, bytes, 0644)
}

func (d *Database) Create(object any) (*Record, error) {
	bytes, err := common.ToBytes(object)
	if err != nil {
		return nil, err
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	offset, err := d.endOffset()
	if err != nil {
		return nil, err
	}
	id := d.idGenerator.next()
	_, exit := d.state.Records[id]
	if exit {
		return nil, fmt.Errorf("record with id %d already exists", id)
	}
	length, err := d.file.WriteAt(bytes, offset)
	if err != nil {
		return nil, err
	}
	record := &Record{id, offset, int64(length)}
	d.state.Records[id] = record
	if err := d.saveState(); err != nil {
		return nil, err
	}
	return record, nil
}

func (d *Database) Read(id int64, object any) (*Record, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	record, exists := d.state.Records[id]
	if !exists {
		return nil, fmt.Errorf("record with id %d not found", id)
	}
	bytes := make([]byte, record.Length)
	_, err := d.file.ReadAt(bytes, record.Offset)
	if err != nil {
		return nil, err
	}
	err = common.FromBytes(bytes, object)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (d *Database) Delete(id int64) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	_, exists := d.state.Records[id]
	if !exists {
		return fmt.Errorf("record with id %d not found", id)
	}
	delete(d.state.Records, id)
	if err := d.saveState(); err != nil {
		return err
	}
	return nil
}

func (d *Database) Update(id int64, object any) (*Record, error) {
	bytes, err := common.ToBytes(object)
	if err != nil {
		return nil, err
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	record, exists := d.state.Records[id]
	if !exists {
		return nil, fmt.Errorf("record with id %d not found", id)
	}
	offset, err := d.endOffset()
	if err != nil {
		return nil, err
	}
	length, err := d.file.WriteAt(bytes, offset)
	if err != nil {
		return nil, err
	}
	record.Offset = offset
	record.Length = int64(length)
	return record, nil
}

func (d *Database) endOffset() (int64, error) {
	return d.file.Seek(0, io.SeekEnd)
}

func DatabaseTest() {
	db := Db("users.db", &Sequence{})
	defer db.Close()

	user := User{"Jan", "Kowalski", 25, true}
	record, err := db.Create(&user)
	fmt.Println(record, err)

	user.IsActive = false
	record, err = db.Update(record.Id, &user)
	fmt.Println(record, err)

	loadedUser := &User{}
	record, err = db.Read(record.Id, loadedUser)
	fmt.Println(loadedUser, record, err)

	err = db.Delete(record.Id)
	fmt.Println(err)
}

type User struct {
	FirstName string
	LastName  string
	Age       int16
	IsActive  bool
}*/
