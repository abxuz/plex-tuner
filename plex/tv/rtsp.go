package tv

import (
	"context"
	"io"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"
	"github.com/deepch/vdk/format/rtspv2"
)

type RTSPStream struct {
	url      string
	r        *io.PipeReader
	w        *io.PipeWriter
	client   *rtspv2.RTSPClient
	timeline time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

func NewRTSPStream(url string) *RTSPStream {
	s := &RTSPStream{url: url}
	s.r, s.w = io.Pipe()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s
}

func (s *RTSPStream) Start() error {
	opt := rtspv2.RTSPClientOptions{
		URL:                s.url,
		DialTimeout:        3 * time.Second,
		ReadWriteTimeout:   3 * time.Second,
		InsecureSkipVerify: true,
	}
	client, err := rtspv2.Dial(opt)
	if err != nil {
		return err
	}
	s.client = client

	go func() {
		defer s.client.Close()
		muxer := mp4f.NewMuxer(nil)
		err = muxer.WriteHeader(s.client.CodecData)
		if err != nil {
			return
		}
		_, init := muxer.GetInit(s.client.CodecData)
		_, err = s.w.Write(init)
		if err != nil {
			return
		}

		keyFrameTimeout := time.NewTimer(20 * time.Second)
		for {
			var packet *av.Packet
			select {
			case <-s.ctx.Done():
				return
			case <-keyFrameTimeout.C:
				return
			case sig := <-s.client.Signals:
				if sig == rtspv2.SignalStreamRTPStop {
					return
				}
			case packet = <-s.client.OutgoingPacketQueue:
			}

			if packet.IsKeyFrame {
				keyFrameTimeout.Reset(20 * time.Second)
			}

			s.timeline += packet.Duration
			packet.Time = s.timeline

			ready, buf, err := muxer.WritePacket(*packet, false)
			if err != nil {
				return
			}

			if ready {
				_, err = s.w.Write(buf)
				if err != nil {
					return
				}
			}
		}

	}()

	return nil
}

func (s *RTSPStream) Read(b []byte) (int, error) {
	return s.r.Read(b)
}

func (s *RTSPStream) Close() error {
	s.cancel()
	s.r.Close()
	return nil
}
