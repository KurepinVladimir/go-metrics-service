package main

import (
	"log"
	"os"
)

func main() {
	log.Fatal("allowed in main.main")
	log.Fatalf("allowed too")
	os.Exit(0)
}
