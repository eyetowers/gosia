package sia

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const (
	maxSequence = 9999
)

type PingError func(err error)

// ClientOption configures optional behaviour on a Client.
type ClientOption func(*Client)

// WithEncryptionKey enables AES-CBC payload encryption (DC-09-2013 p.16).
// key must be 16, 24, or 32 bytes. Both outgoing messages and incoming
// responses are handled with encrypted framing while the option is set.
func WithEncryptionKey(key []byte) ClientOption {
	return func(c *Client) { c.key = key }
}

type Client struct {
	server   string
	identity Identity
	key      []byte // nil means plaintext

	ctx     context.Context
	stop    context.CancelFunc
	workers sync.WaitGroup

	verbose bool

	mu       sync.Mutex
	sequence uint16
}

func New(
	server string, identity Identity, pingPeriod time.Duration, pingError PingError, verbose bool,
	opts ...ClientOption,
) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		server:   server,
		identity: identity,
		ctx:      ctx,
		stop:     cancel,
		verbose:  verbose,
	}
	for _, o := range opts {
		o(c)
	}

	if _, err := linePrefix(identity); err != nil {
		return nil, err
	}

	if err := c.ping(); err != nil {
		return nil, fmt.Errorf("initial SIA ping: %w", err)
	}

	c.workers.Add(1)
	go c.keepAlive(pingPeriod, pingError)

	return c, nil
}

func (c *Client) Send(message Message) error {
	return c.send(c.nextSequence(), message)
}

func (c *Client) ping() error {
	return c.send(c.nextSequence(), empty{id: "NULL", ts: time.Now()})
}

func (c *Client) nextSequence() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sequence++
	if c.sequence > maxSequence {
		c.sequence = 1
	}
	return c.sequence
}

func (c *Client) keepAlive(pingPeriod time.Duration, pingError PingError) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.workers.Done()

	for {
		select {
		case <-ticker.C:
			err := c.ping()
			if err != nil && pingError != nil {
				pingError(err)
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) Close() {
	c.stop()
	c.workers.Wait()
}

func (c *Client) send(sequence uint16, message Message) error {
	var (
		m   string
		err error
	)
	if c.key != nil {
		m, err = EncodeEncrypted(sequence, c.identity, message, c.key)
	} else {
		m, err = Encode(sequence, c.identity, message)
	}
	if err != nil {
		return err
	}

	conn, err := net.Dial("tcp", c.server)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", c.server, err)
	}
	defer conn.Close()

	if c.verbose {
		fmt.Fprintf(os.Stderr, "SENT: %q\n", m)
	}

	_, err = conn.Write([]byte(m))
	if err != nil {
		return fmt.Errorf("sending message %q to %q: %w", m, c.server, err)
	}

	resp, err := bufio.NewReader(conn).ReadString(0x0D)
	if err != nil {
		return fmt.Errorf("reading server %q response: %w", c.server, err)
	}
	if c.verbose {
		fmt.Fprintf(os.Stderr, "GOT: %q\n", resp)
	}

	var parsed ParsedFrame
	if c.key != nil {
		keys := MapKeyStore{c.identity.Account: c.key}
		parsed, err = ParseEncrypted(resp, keys)
	} else {
		parsed, err = Parse(resp)
	}
	if err != nil {
		return fmt.Errorf("parsing server %q response %q: %w", c.server, resp, err)
	}
	return classifyResponse(parsed, sequence, c.identity)
}

// classifyResponse maps a parsed receiver response to either nil (ACK), a
// typed *NakError / *DuhError, or a generic "unexpected reply" error.
func classifyResponse(parsed ParsedFrame, sequence uint16, identity Identity) error {
	if parsed.Sequence != sequence {
		return fmt.Errorf("mismatched sequence %d, expected %d", parsed.Sequence, sequence)
	}
	switch parsed.Message.ID() {
	case Ack.ID():
		if parsed.Account != identity.Account {
			return fmt.Errorf("mismatched account %q, expected %q", parsed.Account, identity.Account)
		}
		return nil
	case Nak.ID():
		return &NakError{
			Receiver: parsed.Receiver,
			Line:     parsed.Line,
			Account:  parsed.Account,
			Sequence: parsed.Sequence,
		}
	case Duh.ID():
		return &DuhError{
			Receiver: parsed.Receiver,
			Line:     parsed.Line,
			Account:  parsed.Account,
			Sequence: parsed.Sequence,
		}
	default:
		return fmt.Errorf("unexpected reply id %q", parsed.Message.ID())
	}
}

// NakError is returned by Client.Send when the receiver answers with a NAK
// frame, indicating protocol-level rejection of the request.
type NakError struct {
	Receiver string
	Line     string
	Account  string
	Sequence uint16
}

func (e *NakError) Error() string {
	return fmt.Sprintf("SIA receiver rejected message (NAK) seq=%d receiver=%q line=%q account=%q",
		e.Sequence, e.Receiver, e.Line, e.Account)
}

func (e *NakError) Is(target error) bool {
	_, ok := target.(*NakError)
	return ok
}

// DuhError is returned by Client.Send when the receiver answers with a DUH
// frame, indicating it does not understand the message id.
type DuhError struct {
	Receiver string
	Line     string
	Account  string
	Sequence uint16
}

func (e *DuhError) Error() string {
	return fmt.Sprintf("SIA receiver did not understand message (DUH) seq=%d receiver=%q line=%q account=%q",
		e.Sequence, e.Receiver, e.Line, e.Account)
}

func (e *DuhError) Is(target error) bool {
	_, ok := target.(*DuhError)
	return ok
}
