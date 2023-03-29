package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/alloydbconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

var conn *pgxpool.Pool

func main() {
	ctx := context.Background()

	// Setup DB connection
	var err error
	conn, err = connectDB(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	createTableIfNotExists(ctx, conn)

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

	err := insertTimestampToDB(context.Background(), trace)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to insert records to DB: %w", err), 500)
	}

	fmt.Fprintf(w, "Success")
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {}

func createTableIfNotExists(ctx context.Context, conn *pgxpool.Pool) {
	_, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS record (id SERIAL PRIMARY KEY, trace_id TEXT, type TEXT, timestamp TIMESTAMP)")
	if err != nil {
		panic(err)
	}
	fmt.Println("Table created successfully")
}

func insertTimestampToDB(ctx context.Context, trace string) error {
	// var err error
	// var tx pgx.Tx

	sql := "INSERT INTO record (trace_id, type, timestamp) VALUES ($1, $2, $3);"
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Print(err)
		return err
	}

	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sql, trace, "BEGIN", time.Now()); err != nil {
		log.Print(err)
		return err
	}
	log.Printf("Begin record inserted successfully: %s", trace)

	// Wait for 5 seconds for testing
	log.Printf("Wait for 5 seconds: %s", trace)
	time.Sleep(5 * time.Second)

	if _, err := tx.Exec(ctx, sql, trace, "END", time.Now()); err != nil {
		log.Print(err)
		return err
	}
	log.Printf("End record inserted successfully: %s", trace)

	if err := tx.Commit(ctx); err != nil {
		log.Print(err)
		return err
	}
	log.Printf("Trasaction commited successfully: %s", trace)

	return nil
}

func connectDB(ctx context.Context) (*pgxpool.Pool, error) {
	var (
		env    = mustGetenv("ENVIRONMENT")
		dbHost = mustGetenv("DB_HOST")
		dbPort = mustGetenv("DB_PORT")
		dbUser = mustGetenv("DB_USER")
		dbPwd  = mustGetenv("DB_PASS")
		dbName = mustGetenv("DB_NAME")
	)
	// Configure the driver to connect to the database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPwd, dbName)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("failed to parse pgx config: %v", err)
	}

	if env == "PROD" {
		log.Print("Using AlloyDB Go Connector dialer")

		// Create a new dialer with any options
		d, err := alloydbconn.NewDialer(ctx)
		if err != nil {
			log.Fatalf("failed to initialize dialer: %v", err)
		}
		defer d.Close()

		// Tell the driver to use the AlloyDB Go Connector to create connections
		config.ConnConfig.DialFunc = func(ctx context.Context, _ string, instance string) (net.Conn, error) {
			return d.Dial(ctx, "projects/<PROJECT>/locations/<REGION>/clusters/<CLUSTER>/instances/<INSTANCE>")
		}
	} else {
		log.Print("Using default dialer")
	}

	// Interact with the driver directly as you normally would
	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	return conn, nil
}
