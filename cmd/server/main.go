package main

import (
	"os"

	"github.com/eyetowers/gosia/pkg/sia"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing the bind address argument.")
	}
	err := sia.Serve(os.Args[1])
	if err != nil {
		panic(err)
	}
}
