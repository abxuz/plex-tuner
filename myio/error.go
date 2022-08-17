package myio

import "errors"

var (
	ErrReadClosedIO       = errors.New("io: read on closed io")
	ErrWriteClosedIO      = errors.New("io: write on closed io")
	ErrChunkIndexOverflow = errors.New("chunk index overflow")
)
