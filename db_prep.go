package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	DSN string = "flibgo:flibgo@tcp(db:3306)/flibgo?charset=utf8"
)

type DB struct {
	*sql.DB
}

func main() {
	db := NewDB(DSN)
	defer db.Close()

	start := time.Now()
	for i := 0; i < 10000; i++ {
		db.FindBook("прив")
	}
	finish := time.Now()
	elapsed := finish.Sub(start)
	fmt.Println("Time elapsed: ", elapsed)

	start = time.Now()
	stmt, err := db.Prepare("SELECT id FROM books WHERE sort LIKE ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < 10000; i++ {
		db.FindBookPrep("прив", stmt)
	}
	finish = time.Now()
	elapsed = finish.Sub(start)
	fmt.Println("Time elapsed: ", elapsed)

}

func NewDB(dsn string) *DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(10)

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return &DB{db}
}

func (db *DB) FindBook(search string) int64 {
	var id int64 = 0
	q := "SELECT id FROM books WHERE sort LIKE ?"
	err := db.QueryRow(q, search).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}

func (db *DB) FindBookPrep(search string, stmt *sql.Stmt) int64 {
	var id int64 = 0

	// q := "SELECT id FROM books WHERE sort LIKE ?"
	// err := db.QueryRow(q, search).Scan(&id)

	err := stmt.QueryRow(search).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}
