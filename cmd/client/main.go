package main

import (
	"fmt"
	"os"
	"time"

	"github.com/eyetowers/gosia/pkg/sia"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing the target address argument.")
	}

	client, err := sia.New(os.Args[1], sia.AuthCode("1120"), 10*time.Second, func(err error) {
		fmt.Printf("Ping error: %s\n", err)
	})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	time.Sleep(15 * time.Second)

	msg := sia.DCS(
		"BA",
		sia.Zone(2, "Zone 2"),
		sia.Area(1, "Partition 1"),
		sia.Verification("https://portal.eyetowers.io"),
		sia.Timestamp(time.Now()),
	)
	err = client.Send(msg)
	if err != nil {
		panic(err)
	}
}
