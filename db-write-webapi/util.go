package main

import (
	"log"
	"os"
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("Warning: %s environment variable not set.", k)
	}
	return v
}
