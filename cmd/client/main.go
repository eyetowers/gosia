package main

import (
	"os"

	"github.com/eyetowers/gosia/pkg/sia"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing the target address argument.")
	}
	i := sia.AuthCode("1120")
	m := sia.DCS("BA", sia.Named(1, "Partition 1"), sia.Named(2, "Zone 2"), sia.Empty())
	err := sia.Send(os.Args[1], 1, i, m)
	if err != nil {
		panic(err)
	}
}
