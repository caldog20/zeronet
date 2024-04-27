package main

import (
	"log"

	"github.com/caldog20/zeronet/controller/cmd"
)

// Calls root cobra command in controller/cmd/root.go
func main() {
	log.Println("starting controller...")
	cmd.Execute()
}
