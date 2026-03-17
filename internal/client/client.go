package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/qfex/cli/internal/protocol"
)

// Client communicates with the qfex daemon over a Unix socket.
type Client struct {
	socketPath string
}

func New(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// Send sends a one-shot request and returns the response.
func (c *Client) Send(ctx context.Context, cmd string, params any) (*protocol.Response, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return c.sendOnConn(ctx, conn, cmd, params)
}

func (c *Client) sendOnConn(ctx context.Context, conn net.Conn, cmd string, params any) (*protocol.Response, error) {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
	}

	req := protocol.Request{Cmd: cmd, Params: rawParams}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	b = append(b, '\n')

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(b); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, fmt.Errorf("connection closed")
	}

	var resp protocol.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

// Watch sends a streaming watch request and calls fn for each event until ctx is done.
func (c *Client) Watch(ctx context.Context, params protocol.WatchParams, fn func(protocol.Event) error) error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send watch request
	req := protocol.Request{Cmd: protocol.CmdWatch}
	req.Params, err = json.Marshal(params)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(req)
	b = append(b, '\n')

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(b); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Stream events
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	defer close(done)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Could be an error response
		var resp protocol.Response
		if err := json.Unmarshal(line, &resp); err == nil && !resp.OK && resp.Error != "" {
			return fmt.Errorf("daemon error: %s", resp.Error)
		}

		var evt protocol.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		if err := fn(evt); err != nil {
			return err
		}
	}

	if ctx.Err() != nil {
		return nil
	}
	return scanner.Err()
}

func (c *Client) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to qfex daemon (%s): %w\nRun 'qfex daemon start' first", c.socketPath, err)
	}
	return conn, nil
}

// IsRunning checks if the daemon is reachable.
func (c *Client) IsRunning() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
