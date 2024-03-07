package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

func main() {
	os.Remove("x.db") // I delete the file to avoid duplicated records.
	// SQLite is a file based database.

	log.Println("Creating sqlite-database.db...")
	file, err := os.Create("x.db") // Create SQLite file
	if err != nil {
		log.Fatal(err.Error())
	}
	file.Close()
	log.Println("done")

	sqliteDatabase, _ := sql.Open("sqlite3", "./x.db") // Open the created SQLite File
	defer sqliteDatabase.Close()                       // Defer Closing the database
	createObjectTable(sqliteDatabase)                  // Create Database Tables

	// INSERT RECORDS
	insertObject(sqliteDatabase, "t1/test", 1000)
	insertObject(sqliteDatabase, "t1/test", 1008)
	insertObject(sqliteDatabase, "t1/test", 1009)
	insertObject(sqliteDatabase, "t1/test", 1009)
	insertObject(sqliteDatabase, "t1/test", 1000)
	insertObject(sqliteDatabase, "t1/test", 1001)
	insertObject(sqliteDatabase, "t1/test", 10065)
	insertObject(sqliteDatabase, "t1/test", 1000)
	insertObject(sqliteDatabase, "t1/test", 1060)
	insertObject(sqliteDatabase, "t1/test", 1000)
	insertObject(sqliteDatabase, "t1/test", 1010)
	insertObject(sqliteDatabase, "t1/test", 1020)
	insertObject(sqliteDatabase, "t1/test", 1006)
	insertObject(sqliteDatabase, "t1/test", 1002)
	insertObject(sqliteDatabase, "t1/test", 1000)

	// DISPLAY INSERTED RECORDS
	displayObjects(sqliteDatabase)
}

func createObjectTable(db *sql.DB) {
	objectTable := `CREATE TABLE objects (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"path" TEXT,
		"size" INT
	  );` // SQL Statement for Create Table

	log.Println("Create object table...")
	statement, err := db.Prepare(objectTable) // Prepare SQL Statement
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec() // Execute SQL Statements
	log.Println("object table created")
}

// We are passing db reference connection from main to our method with other parameters
func insertObject(db *sql.DB, path string, size int) {
	log.Println("Inserting object record ...")
	insertStudentSQL := `INSERT INTO objects(path, size) VALUES (?, ?)`
	statement, err := db.Prepare(insertStudentSQL) // Prepare statement.
	// This is good to avoid SQL injections
	if err != nil {
		log.Fatalln(err.Error())
	}
	_, err = statement.Exec(path, size)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func displayObjects(db *sql.DB) {
	row, err := db.Query("SELECT * FROM objects ORDER BY size")
	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	for row.Next() { // Iterate and fetch the records from result cursor
		var id int
		var path string
		var size int
		row.Scan(&id, &path, &size)
		log.Println("O: ", id, " ", size, " ", path)
	}
}
