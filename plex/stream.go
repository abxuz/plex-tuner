package plex

import (
	"errors"
	"io"
	"net/url"
	"plex-tuner/myio"
	"plex-tuner/plex/tv"
)

type broadcast struct {
	source      tv.TVStream
	piper       *myio.MultiReaderPipe
	readerCount int
}

func (p *Plex) getChannelReader(channel *Channel) (reader io.Reader, release func(), err error) {
	p.broadcastsLock.Lock()
	defer p.broadcastsLock.Unlock()

	key := channel.Type + "-" + channel.URL
	b, exists := p.broadcasts[key]
	if !exists {
		b = &broadcast{}
		b.source, err = p.createTVStream(channel)
		if err != nil {
			return
		}
		err = b.source.Start()
		if err != nil {
			b.source.Close()
			return
		}
		b.piper = myio.NewMultiReaderPipe()
		p.broadcasts[key] = b

		go func() {
			defer b.piper.Close()
			defer b.source.Close()
			io.Copy(b.piper, b.source)
		}()
	}

	consumer := b.piper.PipeReader()
	b.readerCount++
	release = func() {
		consumer.Close()
		p.broadcastsLock.Lock()
		defer p.broadcastsLock.Unlock()

		b.readerCount--
		if b.readerCount == 0 {
			b.piper.Close()
			b.source.Close()
			delete(p.broadcasts, key)
		}
	}
	return consumer, release, nil
}

func (p *Plex) createTVStream(channel *Channel) (tv.TVStream, error) {
	switch channel.Type {
	case "proxy":
		return tv.NewHttpSteam(channel.URL), nil
	case "hls":
		playlistUrl, err := url.Parse(channel.URL)
		if err != nil {
			return nil, err
		}
		return tv.NewHLSStream(playlistUrl), nil
	case "rtsp":
		return tv.NewFFMpegStream(p.config.FFMpeg, channel.URL), nil
	case "bilibili":
		playlistUrl, err := Bilibili.TS(channel.URL)
		if err != nil {
			return nil, err
		}
		channelUrl, err := url.Parse(playlistUrl)
		if err != nil {
			return nil, err
		}
		return tv.NewHLSStream(channelUrl), nil

	}
	return nil, errors.New("unsupport channel type")
}
