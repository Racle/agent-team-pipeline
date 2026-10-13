package main

import (
	"log"
	"net/http"
)

func main() {
	addr := ":8080"
	log.Printf("notes API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, NewServer()))
}
