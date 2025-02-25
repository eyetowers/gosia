package sia

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"

	"github.com/eyetowers/gosia/pkg/sia/message"
)

var (
	messageRE = regexp.MustCompile(`^\n([[:xdigit:]]{8})"([^"]+)"(\d{4})L\d{1,4}#(\d{4})\[.*\]\r$`)
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

	for {
		err := processMessage(c)
		if errors.Is(err, io.EOF) {
			fmt.Printf("[%s] Disconnected\n", c.RemoteAddr())
			return
		}
		if err != nil {
			fmt.Printf("[%s] Error: %s.\n", c.RemoteAddr(), err)
			return
		}
	}
}

func processMessage(c net.Conn) error {
	resp, err := bufio.NewReader(c).ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}
	fmt.Printf("[%s] Received: %q\n", c.RemoteAddr(), resp)

	matches := messageRE.FindStringSubmatch(resp)
	if len(matches) < 5 {
		return fmt.Errorf("unexpected message format %q", resp)
	}
	seq, err := strconv.ParseUint(matches[3], 10, 16)
	if err != nil {
		return fmt.Errorf("malformed message sequence: %w", err)
	}

	m := Encode(uint16(seq), &Identity{AuthCode: matches[4]}, message.Ack)
	fmt.Printf("[%s] Sent: %q\n", c.RemoteAddr(), m)

	_, err = c.Write([]byte(m))
	if err != nil {
		return fmt.Errorf("sending response: %w", err)
	}
	return nil
}
