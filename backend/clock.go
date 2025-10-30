package main

import (
	"context"
	"log"
	"time"
)

type Clock struct {
	ticker   *time.Ticker
	tickChan chan struct{}
	stopChan chan struct{}
}

func NewClock() *Clock {
	return &Clock{
		tickChan: make(chan struct{}),
		stopChan: make(chan struct{}),
	}
}

func (c *Clock) Start(ctx context.Context) {
	c.ticker = time.NewTicker(1 * time.Second)
	defer c.ticker.Stop()

	log.Println("Clock started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-c.ticker.C:
			select {
			case c.tickChan <- struct{}{}:
			default:
			}
		}
	}
}

func (c *Clock) Stop() {
	close(c.stopChan)
}

func (c *Clock) Subscribe() <-chan struct{} {
	return c.tickChan
}