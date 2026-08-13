package main

import (
	"context"
	"log"
	"time"

	gracefulshutdown "github.com/louiswu1110/golang-graceful-shutdown"
)

type pool struct {
	jobs chan int
	done chan struct{}
}

func newPool() *pool { return &pool{jobs: make(chan int), done: make(chan struct{})} }

func (p *pool) Start(ctx context.Context) error {
	defer close(p.done)
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-p.jobs:
			log.Printf("processing job %d", job)
		}
	}
}

func (p *pool) Shutdown(ctx context.Context) error {
	// Stop accepting new jobs here, then wait for workers to finish.
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	manager := gracefulshutdown.New(gracefulshutdown.WithTimeout(10 * time.Second))
	manager.Add(newPool(), gracefulshutdown.WithName("workers"))
	if err := manager.Run(context.Background()); err != nil {
		log.Printf("service stopped: %v", err)
	}
}
