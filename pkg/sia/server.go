package sia

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
)

func Serve(bind string) error {
	l, err := net.Listen("tcp4", bind)
	if err != nil {
		return fmt.Errorf("listening on %q: %w", bind, err)
	}
	defer l.Close()

	fmt.Printf("Listening on %q\n", bind)
	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accepting connection %q on %q: %w", c.RemoteAddr(), bind, err)
		}
		go handleConnection(c)
	}
}

func handleConnection(c net.Conn) {
	fmt.Printf("[%s] Connected\n", c.RemoteAddr())
	defer c.Close()

	peer := c.RemoteAddr().String()
	reader := bufio.NewReader(c)

	for {
		err := processMessage(peer, reader, c)
		if errors.Is(err, io.EOF) {
			fmt.Printf("[%s] Disconnected\n", peer)
			return
		}
		if err != nil {
			fmt.Printf("[%s] Error: %s.\n", peer, err)
			return
		}
	}
}

func processMessage(peer string, r *bufio.Reader, w io.Writer) error {
	req, err := r.ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}
	fmt.Printf("[%s] Received: %q\n", peer, req)

	parsed, err := Parse(req)
	if err != nil {
		return fmt.Errorf("parsing request %q: %w", req, err)
	}

	m, err := Encode(parsed.Sequence, Identity{Account: parsed.Account, Line: parsed.Line}, Ack)
	if err != nil {
		return fmt.Errorf("encoding response: %w", err)
	}
	fmt.Printf("[%s] Sent: %q\n", peer, m)

	_, err = w.Write([]byte(m))
	if err != nil {
		return fmt.Errorf("sending response: %w", err)
	}
	return nil
}
