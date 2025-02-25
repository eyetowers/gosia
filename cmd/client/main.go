package main

import (
	"os"

	"github.com/eyetowers/gosia/pkg/sia"
	"github.com/eyetowers/gosia/pkg/sia/message"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing the target address argument.")
	}
	i := &sia.Identity{
		AuthCode: "1120",
	}
	m := message.DCS("BA", message.Named(1, "Zone 1"), message.Named(2, "Partition 2"), message.Empty())
	err := sia.Send(os.Args[1], 1, i, m)
	if err != nil {
		panic(err)
	}
}
