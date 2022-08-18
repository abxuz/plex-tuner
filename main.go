package main

import (
	"context"
	"log"
	"os/signal"
	"plex-tuner/plex"
	"sync"
	"syscall"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	p := plex.New()
	var once sync.Once

	go func() {
		<-ctx.Done()
		once.Do(p.Close)
	}()

	err := p.Serve(ctx)
	if err != nil {
		log.Fatal(err)
	}
	once.Do(p.Close)
}
