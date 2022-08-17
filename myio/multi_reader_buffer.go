package myio

import (
	"bytes"
	"container/list"
	"context"
	"io"
	"sync"
)

// MultiReaderPipe 单个写，多个读的管道
//
// 内部不带缓冲区，一次数据的写入需要等所有的数据消费者全部取走数据.
type MultiReaderPipe struct {
	consumerList     *list.List
	consumerListLock *sync.RWMutex
	ctx              context.Context
	cancelFn         context.CancelFunc
}

// consumer 数据消费者，实现io.ReadCloser接口
type consumer struct {
	readers       chan io.Reader
	currentReader io.Reader
	ctx           context.Context
	cancelFn      context.CancelFunc

	pipe *MultiReaderPipe
	elem *list.Element
}

// NewMultiReaderPipe 创建一个单个写，多个读的管道
func NewMultiReaderPipe() *MultiReaderPipe {
	p := &MultiReaderPipe{
		consumerList:     list.New(),
		consumerListLock: new(sync.RWMutex),
	}
	p.ctx, p.cancelFn = context.WithCancel(context.Background())
	return p
}

// 实现io.Writer接口，往缓冲区写入数据。
// 该写的操作只有管道被关闭的时候，会返回error
func (p *MultiReaderPipe) Write(data []byte) (int, error) {
	select {
	case <-p.ctx.Done():
		return 0, ErrWriteClosedIO
	default:
	}

	p.consumerListLock.RLock()
	defer p.consumerListLock.RUnlock()

	copyedData := make([]byte, len(data))
	copy(copyedData, data)
	for e := p.consumerList.Front(); e != nil; e = e.Next() {
		c := e.Value.(*consumer)
		reader := bytes.NewReader(copyedData)
		select {
		case c.readers <- reader: // consumer写入成功
		case <-c.ctx.Done(): // consumer被close
		case <-p.ctx.Done(): // pipe被close
			return 0, ErrWriteClosedIO
		}
	}
	return len(data), nil
}

// 从管道中获取一个读的IO
func (p *MultiReaderPipe) PipeReader() *consumer {
	c := &consumer{
		readers: make(chan io.Reader),
		pipe:    p,
	}
	c.ctx, c.cancelFn = context.WithCancel(p.ctx)

	p.consumerListLock.Lock()
	defer p.consumerListLock.Unlock()

	c.elem = p.consumerList.PushBack(c)
	return c
}

// Close 关闭该管道，Close可以安全的被多次调用
func (p *MultiReaderPipe) Close() error {
	// consumer的ctx来自于pipe的context，
	// 所以pipe的context cancel后，consumer的context会自动被cancel
	p.cancelFn()
	// list不清空也没关系
	return nil
}

func (c *consumer) Read(b []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, ErrReadClosedIO
	default:
	}

consumerReadStart:
	if c.currentReader == nil {
		select {
		case c.currentReader = <-c.readers:
		case <-c.ctx.Done():
			return 0, ErrReadClosedIO
		}
	}

	n, err := c.currentReader.Read(b)
	if err == nil {
		if n > 0 {
			return n, nil
		}
		c.currentReader = nil
		goto consumerReadStart
	}

	if err == io.EOF {
		c.currentReader = nil
		if n > 0 {
			return n, nil
		}
		goto consumerReadStart
	}

	return n, err
}

// Close 关闭读取IO，不会再收到从管道中写入的数据，Close可以安全的被多次调用
func (c *consumer) Close() error {
	// 先cancel掉context，让所有的write和read都不再阻塞，可以释放占用的锁
	c.cancelFn()
	c.pipe.consumerListLock.Lock()
	if c.elem != nil {
		c.pipe.consumerList.Remove(c.elem)
	}
	c.pipe.consumerListLock.Unlock()
	return nil
}
