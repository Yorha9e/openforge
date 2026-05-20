package service

import (
	"context"
	"fmt"
)

type Message struct {
	From string
	To   string
	Body []byte
}

type CSPChannel struct {
	name string
	ch   chan Message
}

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
		return fmt.Errorf("channel %s full: backpressure", c.name)
	}
}

func (c *CSPChannel) Receive() <-chan Message {
	return c.ch
}
