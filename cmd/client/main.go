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

	time.Sleep(time.Minute)

	msg := sia.DCS("BA", sia.Named(1, "Partition 1"), sia.Named(2, "Zone 2"), sia.Empty())
	err = client.Send(msg)
	if err != nil {
		panic(err)
	}
}
