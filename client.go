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

// PingErrorHandler is called when a keepalive ping fails.
type PingErrorHandler func(err error)

// Option configures a Client created by Dial.
type Option func(*clientConfig) error

type clientConfig struct {
	pingPeriod time.Duration
	pingError  PingErrorHandler
	verbose    bool
}

// WithKeepalive enables periodic keepalive pings. A zero duration disables
// periodic keepalive pings.
func WithKeepalive(period time.Duration) Option {
	return func(c *clientConfig) error {
		if period < 0 {
			return fmt.Errorf("SIA ping period must be non-negative, got %s", period)
		}
		c.pingPeriod = period
		return nil
	}
}

// WithPingErrorHandler sets the callback used when a keepalive ping fails.
func WithPingErrorHandler(handler PingErrorHandler) Option {
	return func(c *clientConfig) error {
		c.pingError = handler
		return nil
	}
}

// WithVerbose enables protocol logging to stderr.
func WithVerbose() Option {
	return func(c *clientConfig) error {
		c.verbose = true
		return nil
	}
}

// Client sends SIA DC-09 messages to a receiver.
type Client struct {
	server   string
	identity Identity

	ctx     context.Context
	stop    context.CancelFunc
	workers sync.WaitGroup

	verbose bool

	mu       sync.Mutex
	sequence uint16
}

// Dial creates a client, sends an initial ping, and starts periodic keepalive
// pings when configured with WithKeepalive.
func Dial(server string, identity Identity, options ...Option) (*Client, error) {
	cfg := clientConfig{}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		server:   server,
		identity: identity,
		ctx:      ctx,
		stop:     cancel,
		verbose:  cfg.verbose,
	}

	if _, err := linePrefix(identity); err != nil {
		return nil, err
	}

	if err := c.ping(); err != nil {
		return nil, fmt.Errorf("initial SIA ping: %w", err)
	}

	if cfg.pingPeriod > 0 {
		c.workers.Add(1)
		go c.keepAlive(cfg.pingPeriod, cfg.pingError)
	}

	return c, nil
}

// Send encodes and transmits a message to the configured receiver.
func (c *Client) Send(message Message) error {
	return c.send(c.nextSequence(), message)
}

func (c *Client) ping() error {
	return c.send(c.nextSequence(), Null)
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

func (c *Client) keepAlive(pingPeriod time.Duration, pingError PingErrorHandler) {
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

// Close stops the keepalive worker and releases client resources.
func (c *Client) Close() {
	c.stop()
	c.workers.Wait()
}

func (c *Client) send(sequence uint16, message Message) error {
	m, err := Encode(sequence, c.identity, message)
	if err != nil {
		return err
	}

	conn, err := net.Dial("tcp", c.server)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", c.server, err)
	}
	defer func() {
		_ = conn.Close()
	}()

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

	parsed, err := ParseWithKey(resp, c.identity.key())
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
