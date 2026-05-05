package main

import (
	"os"

	sia "github.com/eyetowers/gosia"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing the bind address argument.")
	}
	var err error
	if len(os.Args) >= 3 {
		key, parseErr := sia.ParseEncryptionKey(os.Args[2])
		if parseErr != nil {
			panic(parseErr)
		}
		err = sia.ServeEncrypted(os.Args[1], key)
	} else {
		err = sia.Serve(os.Args[1])
	}
	if err != nil {
		panic(err)
	}
}
