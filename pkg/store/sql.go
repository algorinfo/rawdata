package store

import (
	"fmt"
	"log"

	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

var dataSchema = `
CREATE TABLE IF NOT EXISTS data (
	data_id    TEXT PRIMARY KEY,
    data       BLOB NOT NULL,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- CREATE INDEX  IF NOT EXISTS groupby_ix ON data(group_by);
CREATE INDEX  IF NOT EXISTS created_ix ON data(created_at);

`

func CreateDB(dbName string) *sqlx.DB {
	dbF := fmt.Sprintf("%s.db", dbName)
	db, err := sqlx.Connect("sqlite3", dbF)
	if err != nil {
		log.Fatalln(err)
	}
	db.MustExec(dataSchema)

	return db

}

func Backup(dbSrc, dbDst string) {

	driverName := "sqlite3_with_hook_example"
	sqlite3conn := []*sqlite3.SQLiteConn{}
	sql.Register(driverName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			sqlite3conn = append(sqlite3conn, conn)
			return nil
		},
	})
	dbOrigin, err := sql.Open(driverName, dbSrc)
	if err != nil {
		log.Fatal(err)
	}
	defer dbOrigin.Close()

	if dbOrigin.Ping() != nil {
		log.Fatal("Ping dborigin")
	}

	dbBackup, err := sql.Open(driverName, dbDst)
	if err != nil {
		log.Fatal(err)
	}
	defer dbBackup.Close()
	if dbBackup.Ping() != nil {
		log.Fatal("Ping backup")
	}

	fmt.Println("SQLI CONN", sqlite3conn)

	bk, err := sqlite3conn[1].Backup("main", sqlite3conn[0], "main")
	if err != nil {
		log.Fatal(err)
	}
	defer bk.Close()

	_, err = bk.Step(-1)
	if err != nil {
		log.Fatal(err)
	}
	err = bk.Finish()
	if err != nil {
		log.Fatal("Failed to finish backup:", err)
	}

}
