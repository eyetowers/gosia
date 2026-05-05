package sia

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
)

// Serve starts a TCP SIA DC-09 receiver on bind and acknowledges valid frames.
func Serve(bind string) error {
	return serve(bind, nil)
}

// ServeEncrypted starts a TCP SIA DC-09 receiver that expects encrypted frames.
func ServeEncrypted(bind string, key []byte) error {
	if err := validateAESKey(key); err != nil {
		return err
	}
	if len(key) == 0 {
		return fmt.Errorf("%w: encrypted server requires an AES key", ErrEncryption)
	}
	return serve(bind, append([]byte(nil), key...))
}

func serve(bind string, key []byte) error {
	l, err := net.Listen("tcp4", bind)
	if err != nil {
		return fmt.Errorf("listening on %q: %w", bind, err)
	}
	defer func() {
		_ = l.Close()
	}()

	fmt.Printf("Listening on %q\n", bind)
	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accepting connection on %q: %w", bind, err)
		}
		go handleConnection(c, key)
	}
}

func handleConnection(c net.Conn, key []byte) {
	fmt.Printf("[%s] Connected\n", c.RemoteAddr())
	defer func() {
		_ = c.Close()
	}()

	peer := c.RemoteAddr().String()
	reader := bufio.NewReader(c)

	for {
		err := processMessage(peer, reader, c, key)
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

func processMessage(peer string, r *bufio.Reader, w io.Writer, key []byte) error {
	req, err := r.ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}
	fmt.Printf("[%s] Received: %q\n", peer, req)

	parsed, err := ParseWithKey(req, key)
	if err != nil {
		return fmt.Errorf("parsing request %q: %w", req, err)
	}

	identity := Identity{Account: parsed.Account, Line: parsed.Line}
	if parsed.Encrypted {
		identity = identity.WithEncryptionKey(key)
	}
	m, err := Encode(parsed.Sequence, identity, Ack)
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
