package main

import (
	"context"
	"fmt"
	"os"
	"time"

	sia "github.com/eyetowers/gosia"
)

func main() {
	if len(os.Args) < 3 {
		panic("Missing the target address argument.")
	}

	identity := sia.Account(os.Args[2])
	if len(os.Args) >= 4 {
		var err error
		identity, err = identity.WithEncryptionKeyHex(os.Args[3])
		if err != nil {
			panic(err)
		}
	}
	ctx := context.Background()

	client, err := sia.Dial(
		ctx,
		os.Args[1],
		identity,
		sia.WithKeepalive(10*time.Second),
		sia.WithTimeout(10*time.Second),
		sia.WithPingErrorHandler(func(err error) {
			fmt.Printf("Ping error: %s\n", err)
		}),
		sia.WithVerbose(),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	err = client.Send(ctx, sia.Event(
		"RP",
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	err = client.Send(ctx, sia.Event(
		"OA",
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	err = client.Send(ctx, sia.Event(
		"CG",
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)

	err = client.Send(ctx, sia.Event(
		"BA",
		sia.Zone(2, "Zone 2"),
		sia.Area(1, "Partition 1"),
		sia.Verification("https://example.com/verification"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)

	err = client.Send(ctx, sia.Event(
		"BR",
		sia.Zone(2, "Zone 2"),
		sia.Area(1, "Partition 1"),
		sia.Timestamp(time.Now()),
	))
	if err != nil {
		panic(err)
	}
}
