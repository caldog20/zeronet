package main

import (
	"github.com/caldog20/zeronet/node/cmd"
)

// Calls root cobra command in controller/cmd/root.go
func main() {
	cmd.Execute()
}
