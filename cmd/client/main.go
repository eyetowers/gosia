package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/eyetowers/gosia/pkg/sia"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: client <server> <account> [<key>]")
		os.Exit(1)
	}

	var opts []sia.ClientOption
	if len(os.Args) >= 4 {
		key, err := hex.DecodeString(os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "key must be a hex string: %v\n", err)
			os.Exit(1)
		}
		if n := len(key); n != 16 && n != 24 && n != 32 {
			fmt.Fprintf(os.Stderr, "key must decode to 16, 24, or 32 bytes (got %d)\n", n)
			os.Exit(1)
		}
		opts = append(opts, sia.WithEncryptionKey(key))
	}

	client, err := sia.New(os.Args[1], sia.Account(os.Args[2]), 10*time.Second, func(err error) {
		fmt.Printf("Ping error: %s\n", err)
	}, true, opts...)
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
