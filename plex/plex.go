package plex

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

const Version = "1.0.5"

type Plex struct {
	config *Config
	ctx    context.Context
	cancel context.CancelFunc

	logWriter io.Writer
	logger    *log.Logger
	server    *http.Server

	broadcasts     map[string]*broadcast
	broadcastsLock *sync.Mutex
}

func New() *Plex {
	p := &Plex{
		broadcasts:     make(map[string]*broadcast),
		broadcastsLock: new(sync.Mutex),
	}
	return p
}

func (p *Plex) Serve(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	var config string
	flag.StringVar(&config, "config", "config.json", "config file path")
	flag.Parse()

	c, err := loadConfig(config)
	if err != nil {
		return err
	}
	p.config = c
	if p.config.Log == "" {
		p.logWriter = io.Discard
	} else {
		logFile, err := os.OpenFile(p.config.Log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		p.logWriter = logFile
	}
	p.logger = log.New(p.logWriter, "plex-tuner", log.LstdFlags)
	p.server = &http.Server{
		Addr:     p.config.Listen,
		Handler:  p.newHttpHandler(),
		ErrorLog: p.logger,
	}
	return p.server.ListenAndServe()
}

func (p *Plex) Close() {
	p.cancel()
	p.server.Close()
	if closer, ok := p.logWriter.(io.Closer); ok {
		closer.Close()
	}
}
