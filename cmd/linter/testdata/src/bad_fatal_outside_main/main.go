package main

import "log"

func helper() {
	log.Fatal("nope")  // want "log.Fatal forbidden outside main.main"
	log.Fatalf("nope") // want "log.Fatalf forbidden outside main.main"
}

func main() {}
