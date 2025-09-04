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

func New(
	server string, identity Identity, pingPeriod time.Duration, pingError PingError, verbose bool,
) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		server:   server,
		identity: identity,
		ctx:      ctx,
		stop:     cancel,
		verbose:  verbose,
	}

	err := c.ping()
	if err != nil {
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
	return c.send(0, Null)
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
	conn, err := net.Dial("tcp", c.server)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", c.server, err)
	}
	defer conn.Close()

	m := Encode(sequence, c.identity, message)
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

	reply, id, seq, err := Parse(resp)
	if err != nil {
		return fmt.Errorf("parsing server %q response %q: %w", c.server, resp, err)
	}
	if id != c.identity {
		return fmt.Errorf("mismatched identity %q, expected %q", id, c.identity)
	}
	if seq != sequence {
		return fmt.Errorf("mismatched sequence %d, expected %d", seq, sequence)
	}
	if reply.ID() != Ack.ID() {
		return fmt.Errorf("message not acked, got %q instead", reply.ID())
	}
	return nil
}
