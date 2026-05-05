package sia

import (
	"bufio"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestNewRejectsNegativePingPeriod(t *testing.T) {
	_, err := Dial("127.0.0.1:1", Account("1234"), WithKeepalive(-time.Second))
	if err == nil {
		t.Fatal("Dial returned nil error for negative ping period")
	}
}

func TestClientSendAcknowledged(t *testing.T) {
	addr, received := startTestReceiver(t, nil, 2)

	client, err := Dial(addr, Account("1234"))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	err = client.Send(Event(
		"BA",
		Zone(2, "Front Door"),
		Area(1, "Main"),
		Timestamp(time.Now()),
	))
	if err != nil {
		t.Fatalf("Send = %v", err)
	}

	initial := <-received
	if initial.Message.ID() != Null.ID() {
		t.Fatalf("initial message id = %q, want %q", initial.Message.ID(), Null.ID())
	}

	sent := <-received
	if sent.Message.ID() != "SIA-DCS" {
		t.Fatalf("sent message id = %q, want SIA-DCS", sent.Message.ID())
	}
	if sent.Account != "1234" {
		t.Fatalf("sent account = %q, want 1234", sent.Account)
	}
}

func TestClientSendEncryptedAcknowledged(t *testing.T) {
	key := []byte("0123456789ABCDEF")
	addr, received := startTestReceiver(t, key, 2)

	client, err := Dial(addr, Account("1234").WithEncryptionKey(key))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	err = client.Send(Event(
		"RP",
		Timestamp(time.Now()),
	))
	if err != nil {
		t.Fatalf("Send = %v", err)
	}

	<-received
	sent := <-received
	if !sent.Encrypted {
		t.Fatal("sent message was not parsed as encrypted")
	}
	if sent.Message.ID() != "SIA-DCS" {
		t.Fatalf("sent message id = %q, want SIA-DCS", sent.Message.ID())
	}
}

func startTestReceiver(t *testing.T, key []byte, want int) (string, <-chan ParsedFrame) {
	t.Helper()

	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen = %v", err)
	}

	received := make(chan ParsedFrame, want)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer close(received)
		defer func() {
			_ = l.Close()
		}()

		for range want {
			c, err := l.Accept()
			if err != nil {
				return
			}
			processTestConnection(t, c, key, received)
		}
	}()

	t.Cleanup(func() {
		_ = l.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("test receiver did not stop")
		}
	})

	return l.Addr().String(), received
}

func processTestConnection(t *testing.T, c net.Conn, key []byte, received chan<- ParsedFrame) {
	t.Helper()
	defer func() {
		_ = c.Close()
	}()

	req, err := bufio.NewReader(c).ReadString(0x0D)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			t.Errorf("ReadString = %v", err)
		}
		return
	}

	parsed, err := ParseWithKey(req, key)
	if err != nil {
		t.Errorf("ParseWithKey = %v", err)
		return
	}
	received <- parsed

	identity := Identity{Account: parsed.Account, Line: parsed.Line}
	if parsed.Encrypted {
		identity = identity.WithEncryptionKey(key)
	}
	resp, err := Encode(parsed.Sequence, identity, Ack)
	if err != nil {
		t.Errorf("Encode ACK = %v", err)
		return
	}
	if _, err := c.Write([]byte(resp)); err != nil {
		t.Errorf("Write ACK = %v", err)
	}
}
