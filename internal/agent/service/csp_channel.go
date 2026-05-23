package service

import (
	"context"
	"fmt"
	"sync"
)

type Message struct {
	From string
	To   string
	Body []byte
}

type walEntry struct {
	msg  Message
	next *walEntry
}

type CSPChannel struct {
	name    string
	ch      chan Message
	mu      sync.Mutex
	walHead *walEntry
	walTail *walEntry
	walLen  int
}

const maxWALSize = 4096

func NewCSPChannel(name string, bufferSize int) *CSPChannel {
	return &CSPChannel{
		name: name,
		ch:   make(chan Message, bufferSize),
	}
}

func (c *CSPChannel) Send(ctx context.Context, msg Message) error {
	select {
	case c.ch <- msg:
		return nil
	default:
	}

	c.mu.Lock()
	if c.walLen >= maxWALSize {
		c.mu.Unlock()
		return fmt.Errorf("channel %s full: WAL overflow (%d entries)", c.name, c.walLen)
	}
	entry := &walEntry{msg: msg}
	if c.walTail != nil {
		c.walTail.next = entry
	} else {
		c.walHead = entry
	}
	c.walTail = entry
	c.walLen++
	c.mu.Unlock()
	return nil
}

func (c *CSPChannel) Drain(ctx context.Context) {
	c.mu.Lock()
	entry := c.walHead
	c.walHead = nil
	c.walTail = nil
	c.walLen = 0
	c.mu.Unlock()

	for entry != nil {
		select {
		case c.ch <- entry.msg:
		case <-ctx.Done():
			return
		}
		entry = entry.next
	}
}

func (c *CSPChannel) WALLen() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.walLen
}

func (c *CSPChannel) Receive() <-chan Message {
	return c.ch
}
