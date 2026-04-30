package sia

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
)

// Serve listens on bind and handles plain DC-09 frames.
func Serve(bind string) error { return serve(bind, nil) }

// ServeEncrypted listens on bind and handles DC-09 encrypted frames using keys
// to look up per-account AES keys. Responses are sent as encrypted "*ACK" frames.
func ServeEncrypted(bind string, keys KeyStore) error { return serve(bind, keys) }

func serve(bind string, keys KeyStore) error {
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
		go handleConnection(c, keys)
	}
}

func handleConnection(c net.Conn, keys KeyStore) {
	fmt.Printf("[%s] Connected\n", c.RemoteAddr())
	defer c.Close()

	peer := c.RemoteAddr().String()
	reader := bufio.NewReader(c)

	for {
		err := processMessage(peer, reader, c, keys)
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

func processMessage(peer string, r *bufio.Reader, w io.Writer, keys KeyStore) error {
	req, err := r.ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}
	fmt.Printf("[%s] Received: %q\n", peer, req)

	var parsed ParsedFrame
	if keys != nil {
		parsed, err = ParseEncrypted(req, keys)
	} else {
		parsed, err = Parse(req)
	}
	if err != nil {
		return fmt.Errorf("parsing request %q: %w", req, err)
	}

	identity := Identity{Account: parsed.Account, Line: parsed.Line}
	var m string
	if parsed.Encrypted {
		key, ok := keys.LookupKey(parsed.Account)
		if !ok {
			return fmt.Errorf("no key for account %q when encoding response", parsed.Account)
		}
		m, err = EncodeEncrypted(parsed.Sequence, identity, Ack, key)
	} else {
		m, err = Encode(parsed.Sequence, identity, Ack)
	}
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
