package myio

import (
	"bytes"
	"io"
	"sync"
)

type ChunkIO struct {
	index  int
	chunks []io.Reader
	cond   *sync.Cond
	closed bool
}

func NewChunkIO(size int) *ChunkIO {
	c := &ChunkIO{}
	c.chunks = make([]io.Reader, size)
	c.cond = sync.NewCond(new(sync.Mutex))
	return c
}

func (c *ChunkIO) Read(data []byte) (int, error) {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	defer c.cond.Signal() // 防止有多个go程在read

ChunkIOReadStart:
	for c.chunks[c.index] == nil && !c.closed {
		c.cond.Wait()
	}

	if c.closed {
		return 0, ErrReadClosedIO
	}

	n, err := c.chunks[c.index].Read(data)

	// 读取过程中没有出现什么问题
	if err == nil {
		if n > 0 {
			return n, nil
		}
		// n == 0时，认为本次Read什么都没读到，就继续等下次的数据(最后一块chunk时返回eof)
		if c.index+1 == len(c.chunks) {
			return n, io.EOF
		}
		// 不是最后一块，则移到下一块，做等待
		c.index++
		goto ChunkIOReadStart
	}

	// 当前块的数据读完了
	if err == io.EOF {
		// 看是否是最后一块数据，如果是，则返回eof
		if c.index+1 == len(c.chunks) {
			return n, io.EOF
		}

		// 不是最后一块数据，则挪到下一块
		c.index++
		if n > 0 {
			return n, nil
		}
		goto ChunkIOReadStart
	}

	// 出现读取错误
	return n, err
}

func (c *ChunkIO) FillChunk(index int, data []byte) error {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	if index >= len(c.chunks) {
		return ErrChunkIndexOverflow
	}

	copyedData := make([]byte, len(data))
	copy(copyedData, data)
	c.chunks[index] = bytes.NewReader(copyedData)
	c.cond.Signal()
	return nil
}

func (c *ChunkIO) ZeroCopyFillChunk(index int, data []byte) error {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	if index >= len(c.chunks) {
		return ErrChunkIndexOverflow
	}

	c.chunks[index] = bytes.NewReader(data)
	c.cond.Signal()
	return nil
}

func (c *ChunkIO) Close() error {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	c.closed = true
	c.cond.Signal()
	return nil
}
