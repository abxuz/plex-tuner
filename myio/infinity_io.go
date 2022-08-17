package myio

import (
	"bytes"
	"context"
	"io"
)

type InfinityIO struct {
	readers       chan io.Reader
	currentReader io.Reader
	ctx           context.Context
	cancelFn      context.CancelFunc
}

func NewInfinityIO() *InfinityIO {
	i := &InfinityIO{
		readers: make(chan io.Reader),
	}
	i.ctx, i.cancelFn = context.WithCancel(context.Background())
	return i
}

func (i *InfinityIO) Write(b []byte) (int, error) {
	copyedData := make([]byte, len(b))
	copy(copyedData, b)
	select {
	case i.readers <- bytes.NewReader(copyedData):
	case <-i.ctx.Done():
		return 0, ErrWriteClosedIO
	}
	return len(b), nil
}

func (i *InfinityIO) Read(b []byte) (int, error) {
	select {
	case <-i.ctx.Done():
		return 0, ErrReadClosedIO
	default:
	}

InfinityIOReadStart:
	if i.currentReader == nil {
		select {
		case i.currentReader = <-i.readers:
		case <-i.ctx.Done():
			return 0, ErrReadClosedIO
		}
	}

	n, err := i.currentReader.Read(b)
	if err == nil {
		if n > 0 {
			return n, nil
		}
		i.currentReader = nil
		goto InfinityIOReadStart
	}

	if err == io.EOF {
		i.currentReader = nil
		if n > 0 {
			return n, nil
		}
		goto InfinityIOReadStart
	}

	return n, err
}

func (i *InfinityIO) Close() error {
	i.cancelFn()
	return nil
}
