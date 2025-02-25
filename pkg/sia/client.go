package sia

import (
	"bufio"
	"fmt"
	"net"

	"github.com/eyetowers/gosia/pkg/sia/message"
)

func Send(server string, sequence uint16, identity *Identity, message message.Message) error {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", server, err)
	}
	defer conn.Close()
	m := Encode(sequence, identity, message)
	fmt.Printf("msg: %q\n", m)
	_, err = conn.Write([]byte(m))
	if err != nil {
		return fmt.Errorf("sending message %q to %q: %w", m, server, err)
	}
	resp, err := bufio.NewReader(conn).ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading server %q response: %w", server, err)
	}
	fmt.Printf("resp: %q\n", resp)
	return nil
}
