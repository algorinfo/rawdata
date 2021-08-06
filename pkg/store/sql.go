package store

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var dataSchema = `
CREATE TABLE IF NOT EXISTS data (
	data_id    TEXT PRIMARY KEY,
    data       BLOB NOT NULL,
    group_by   TEXT,
    checksum   TEXT,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX  IF NOT EXISTS groupby_ix ON data(group_by);
CREATE INDEX  IF NOT EXISTS created_ix ON data(created_at);

`

func UseDB() {

	db, err := sqlx.Connect("sqlite3", "test.db")
	if err != nil {
		log.Fatalln(err)
	}
	db.MustExec(dataSchema)
	db.MustExec("INSERT INTO data (data_id, data, group_by) VALUES ($1, $2, $3)", "Jason2", []byte("pepe"), "Moiron")

}

func CreateDB(dbName string) *sqlx.DB {
	dbF := fmt.Sprintf("%s.db", dbName)
	db, err := sqlx.Connect("sqlite3", dbF)
	if err != nil {
		log.Fatalln(err)
	}
	db.MustExec(dataSchema)

	return db

}
