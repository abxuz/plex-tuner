package tv

import "io"

type TVStream interface {
	io.ReadCloser
	Start() error
}
