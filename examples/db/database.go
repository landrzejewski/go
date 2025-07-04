package db

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"training.pl/go/common"
)

const stateFileSuffix = ".state"

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
	file, err := os.OpenFile(filepath, os.O_CREATE, 0666)
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

func (d *Database) endOffset() (int64, error) {
	return d.file.Seek(0, io.SeekEnd)
}

func DatabaseTest() {
	db := Db("users.db", &Sequence{})
	db.Close()
}
