package main

import (
	"fmt"
	"os"
	"time"

	"github.com/eyetowers/gosia/pkg/sia"
)

func main() {
	if len(os.Args) < 3 {
		panic("Missing the target address argument.")
	}

	client, err := sia.New(os.Args[1], sia.Account(os.Args[2]), 10*time.Second, func(err error) {
		fmt.Printf("Ping error: %s\n", err)
	}, true)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	err = client.Send(sia.DCS(
		"RP",
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	err = client.Send(sia.DCS(
		"OA",
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	err = client.Send(sia.DCS(
		"CG",
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)

	err = client.Send(sia.DCS(
		"BA",
		sia.Zone(2, "Zone 2"),
		sia.Area(1, "Partition 1"),
		sia.Verification("https://portal.eyetowers.io"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)

	err = client.Send(sia.DCS(
		"BR",
		sia.Zone(2, "Zone 2"),
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}
}
