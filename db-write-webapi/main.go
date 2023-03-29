package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
)

var db *sql.DB

func main() {
	// Setup DB connection
	var err error
	db, err = connectTCPSocket()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTableIfNotExists(db)

	log.Print("Starting server...")
	http.HandleFunc("/", handler)
	http.HandleFunc("/favicon.ico", faviconHandler)

	// Determine port for HTTP service
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	// Start HTTP server
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	projectId := os.Getenv("PROJECT_ID")

	var trace string
	if projectId != "" {
		traceHeader := r.Header.Get("X-Cloud-Trace-Context")
		traceParts := strings.Split(traceHeader, "/")
		if len(traceParts) > 0 && len(traceParts[0]) > 0 {
			trace = fmt.Sprintf("projects/%s/traces/%s", projectId, traceParts[0])
		}
	}

	err := insertTimestampToDB(trace)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to insert records to DB: %w", err), 500)
	}

	fmt.Fprintf(w, "Success")
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {}

func createTableIfNotExists(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS record (id SERIAL PRIMARY KEY, trace_id TEXT, type TEXT, timestamp TIMESTAMP)")
	if err != nil {
		panic(err)
	}
	fmt.Println("Table created successfully")
}

func insertTimestampToDB(trace string) error {
	var err error
	var tx *sql.Tx

	sql := "INSERT INTO record (trace_id, type, timestamp) VALUES ($1, $2, $3);"
	tx, err = db.Begin()
	if err != nil {
		log.Print(err)
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(sql, trace, "BEGIN", time.Now())
	if err != nil {
		log.Print(err)
		return err
	}
	log.Printf("Begin record inserted successfully: %s", trace)

	// Wait for 5 seconds
	log.Printf("Wait for 5 seconds: %s", trace)
	time.Sleep(5 * time.Second)

	_, err = tx.Exec(sql, trace, "END", time.Now())
	if err != nil {
		log.Print(err)
		return err
	}
	log.Printf("End record inserted successfully: %s", trace)

	err = tx.Commit()
	if err != nil {
		log.Print(err)
		return err
	}
	log.Printf("Trasaction commited successfully: %s", trace)

	return nil
}

func connectTCPSocket() (*sql.DB, error) {
	var (
		dbUser    = mustGetenv("DB_USER")
		dbPwd     = mustGetenv("DB_PASS")
		dbTCPHost = mustGetenv("DB_HOST")
		dbPort    = mustGetenv("DB_PORT")
		dbName    = mustGetenv("DB_NAME")
	)

	dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s",
		dbTCPHost, dbUser, dbPwd, dbPort, dbName)

	dbPool, err := sql.Open("pgx", dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %v", err)
	}

	return dbPool, nil
}
