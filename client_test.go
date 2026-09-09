package sia

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestNewRejectsNegativePingPeriod(t *testing.T) {
	_, err := Dial(context.Background(), "127.0.0.1:1", Account("1234"), WithKeepalive(-time.Second))
	if err == nil {
		t.Fatal("Dial returned nil error for negative ping period")
	}
}

func TestDialRejectsNegativeTimeout(t *testing.T) {
	_, err := Dial(context.Background(), "127.0.0.1:1", Account("1234"), WithTimeout(-time.Second))
	if err == nil {
		t.Fatal("Dial returned nil error for negative timeout")
	}
}

func TestDialUsesDefaultTimeout(t *testing.T) {
	addr, _ := startTestReceiver(t, nil, 1)
	client, err := Dial(context.Background(), addr, Account("1234"))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	if client.requestTimeout != defaultRequestTimeout {
		t.Fatalf("request timeout = %s, want %s", client.requestTimeout, defaultRequestTimeout)
	}
}

func TestKeepaliveContinuesAfterRequestTimeout(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen = %v", err)
	}
	defer func() {
		_ = l.Close()
	}()

	stalled := make(chan struct{})
	recovered := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		ackTestConnection(t, l)

		conn, _ := readTestConnection(t, l)
		if conn == nil {
			return
		}
		close(stalled)
		_, _ = io.Copy(io.Discard, conn)
		_ = conn.Close()

		ackTestConnection(t, l)
		close(recovered)
	}()

	pingErrors := make(chan error, 1)
	client, err := Dial(
		context.Background(),
		l.Addr().String(),
		Account("1234"),
		WithKeepalive(200*time.Millisecond),
		WithTimeout(100*time.Millisecond),
		WithPingErrorHandler(func(err error) {
			select {
			case pingErrors <- err:
			default:
			}
		}),
	)
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	waitForSignal(t, stalled, "stalled keepalive")
	select {
	case err := <-pingErrors:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ping error = %v, want context deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for keepalive error")
	}
	waitForSignal(t, recovered, "recovered keepalive")
	waitForSignal(t, serverDone, "test receiver shutdown")
}

func TestCloseCancelsStalledKeepalive(t *testing.T) {
	addr, stalled := startStallingTestReceiver(t)
	client, err := Dial(
		context.Background(),
		addr,
		Account("1234"),
		WithKeepalive(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	waitForSignal(t, stalled, "stalled keepalive")

	closed := make(chan struct{})
	go func() {
		client.Close()
		close(closed)
	}()
	waitForSignal(t, closed, "client close")
}

func TestSendStopsWhenCallerContextIsCanceled(t *testing.T) {
	addr, stalled := startStallingTestReceiver(t)
	client, err := Dial(context.Background(), addr, Account("1234"))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sent := make(chan error, 1)
	go func() {
		sent <- client.Send(ctx, Event("RP"))
	}()
	waitForSignal(t, stalled, "stalled send")
	cancel()

	select {
	case err := <-sent:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Send = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Send to stop")
	}
}

func TestSendAfterCloseReturnsContextCanceled(t *testing.T) {
	addr, _ := startTestReceiver(t, nil, 1)
	client, err := Dial(context.Background(), addr, Account("1234"))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	client.Close()

	err = client.Send(context.Background(), Event("RP"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send after Close = %v, want context canceled", err)
	}
}

func TestClientSendAcknowledged(t *testing.T) {
	addr, received := startTestReceiver(t, nil, 2)

	client, err := Dial(context.Background(), addr, Account("1234"))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	err = client.Send(context.Background(), Event(
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

	client, err := Dial(context.Background(), addr, Account("1234").WithEncryptionKey(key))
	if err != nil {
		t.Fatalf("Dial = %v", err)
	}
	defer client.Close()

	err = client.Send(context.Background(), Event(
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

func readTestConnection(t *testing.T, l net.Listener) (net.Conn, ParsedFrame) {
	t.Helper()

	c, err := l.Accept()
	if err != nil {
		t.Errorf("Accept = %v", err)
		return nil, ParsedFrame{}
	}
	req, err := bufio.NewReader(c).ReadString(0x0D)
	if err != nil {
		t.Errorf("ReadString = %v", err)
		_ = c.Close()
		return nil, ParsedFrame{}
	}
	parsed, err := Parse(req)
	if err != nil {
		t.Errorf("Parse = %v", err)
		_ = c.Close()
		return nil, ParsedFrame{}
	}
	return c, parsed
}

func startStallingTestReceiver(t *testing.T) (string, <-chan struct{}) {
	t.Helper()

	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen = %v", err)
	}
	stalled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		ackTestConnection(t, l)
		conn, _ := readTestConnection(t, l)
		if conn == nil {
			return
		}
		close(stalled)
		_, _ = io.Copy(io.Discard, conn)
		_ = conn.Close()
	}()
	t.Cleanup(func() {
		_ = l.Close()
		waitForSignal(t, done, "test receiver shutdown")
	})
	return l.Addr().String(), stalled
}

func ackTestConnection(t *testing.T, l net.Listener) {
	t.Helper()

	c, parsed := readTestConnection(t, l)
	if c == nil {
		return
	}
	defer func() {
		_ = c.Close()
	}()
	resp, err := Encode(parsed.Sequence, Identity{Account: parsed.Account, Line: parsed.Line}, Ack)
	if err != nil {
		t.Errorf("Encode ACK = %v", err)
		return
	}
	if _, err := c.Write([]byte(resp)); err != nil {
		t.Errorf("Write ACK = %v", err)
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}
