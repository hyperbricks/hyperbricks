package database

import (
	"database/sql"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var once sync.Once

// GetDB returns the singleton database connection
func GetDB() *sql.DB {
	once.Do(func() {
		var err error
		db, err = sql.Open("sqlite3", "./source/resources/database/test.db")
		if err != nil {
			log.Fatal(err)
		}

		// Optional: You can check if the database is reachable
		if err := db.Ping(); err != nil {
			log.Fatal(err)
		}
	})

	return db
}
