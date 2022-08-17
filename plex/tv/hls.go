package tv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"plex-tuner/myio"
	"strings"
	"time"

	"github.com/grafov/m3u8"
	"golang.org/x/sync/errgroup"
)

var (
	ErrUnkownM3u8PlaylistType = errors.New("unkown m3u8 playlist type")
	ErrReadClosedStream       = errors.New("io: read/write on closed stream")
)

const MAX_DOWNLOADER = 5

type HLSStream struct {
	playlistUrl      *url.URL
	lastSegmentSeqId uint64
	loopErr          error

	chunkChan    chan *myio.ChunkIO
	currentChunk *myio.ChunkIO
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewHLSStream(playlistUrl *url.URL) *HLSStream {
	s := &HLSStream{
		playlistUrl: playlistUrl,
		chunkChan:   make(chan *myio.ChunkIO, MAX_DOWNLOADER),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s
}

func (s *HLSStream) Start() error {
	go s.loopLoadSegmentData()
	return nil
}

func (s *HLSStream) Read(b []byte) (int, error) {
	select {
	case <-s.ctx.Done():
		return 0, ErrReadClosedStream
	default:
	}

HLSStreamReadStart:
	if s.currentChunk == nil {
		select {
		case chunk := <-s.chunkChan:
			if s.loopErr != nil {
				return 0, s.loopErr
			}
			s.currentChunk = chunk
		case <-s.ctx.Done():
			return 0, ErrReadClosedStream
		}
	}

	n, err := s.currentChunk.Read(b)
	if err == nil {
		if n > 0 {
			return n, nil
		}
		s.currentChunk = nil
		goto HLSStreamReadStart
	}

	if err == io.EOF {
		s.currentChunk = nil
		if n > 0 {
			return n, nil
		}
		goto HLSStreamReadStart
	}

	return n, err
}

func (s *HLSStream) Close() error {
	s.cancel()
	return nil
}

func (s *HLSStream) loopLoadSegmentData() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		var playlist *m3u8.MediaPlaylist
		playlist, s.loopErr = s.fetchPlaylist()
		if s.loopErr != nil {
			close(s.chunkChan)
			return
		}

		startTime := time.Now()

		extMapData := make([]byte, 0)
		if playlist.Map != nil && playlist.Map.URI != "" {
			var extMapUrl *url.URL
			extMapUrl, s.loopErr = s.parseSegmentUrl(playlist.Map.URI)
			if s.loopErr != nil {
				close(s.chunkChan)
				return
			}
			extMapData, s.loopErr = s.fetchMapData(extMapUrl.String())
			if s.loopErr != nil {
				close(s.chunkChan)
				return
			}
		}

		firstSegmentDuration := time.Duration(0)
		segmentUrls := make([]*url.URL, 0)
		for _, segment := range playlist.Segments {
			if segment == nil {
				continue
			}
			if firstSegmentDuration == 0 {
				firstSegmentDuration = time.Second * time.Duration(segment.Duration)
			}
			if segment.SeqId <= s.lastSegmentSeqId {
				continue
			}
			var segmentUrl *url.URL
			segmentUrl, s.loopErr = s.parseSegmentUrl(segment.URI)
			if s.loopErr != nil {
				close(s.chunkChan)
				return
			}
			segmentUrls = append(segmentUrls, segmentUrl)
			s.lastSegmentSeqId = segment.SeqId
		}

		if len(segmentUrls) > 0 {
			s.loopErr = s.batchDownloadSegments(extMapData, segmentUrls)
			if s.loopErr != nil {
				close(s.chunkChan)
				return
			}
		}

		var sleepDuration time.Duration = 0
		// 整个列表返回的为空，则休眠一秒
		if firstSegmentDuration == 0 {
			sleepDuration = time.Second
		} else {
			// 如果处理整个列表的时间还不到第一段的间隔时间，则休眠第一段的间隔时间
			processDuration := time.Since(startTime)
			if processDuration < firstSegmentDuration {
				sleepDuration = firstSegmentDuration
			} else {
				sleepDuration = 0
			}
		}

		if sleepDuration > 0 {
			timer := time.NewTimer(sleepDuration)
			select {
			case <-timer.C:
			case <-s.ctx.Done():
				return
			}
		}
	}
}

func (s *HLSStream) batchDownloadSegments(extMapData []byte, urls []*url.URL) error {
	chunk := myio.NewChunkIO(len(urls))
	eg, ctx := errgroup.WithContext(s.ctx)
	eg.SetLimit(MAX_DOWNLOADER)
	for i, url := range urls {
		eg.Go(s.chunkDownloader(ctx, extMapData, chunk, i, url.String()))
	}

	// 提前放进去，若后续获取数据出现错误，则关闭chunk，read处则会返回error
	select {
	case s.chunkChan <- chunk:
	case <-s.ctx.Done():
		return nil
	}

	if err := eg.Wait(); err != nil {
		chunk.Close()
		return err
	}
	return nil
}

func (s *HLSStream) parseSegmentUrl(uri string) (*url.URL, error) {
	if strings.HasPrefix(uri, "http://") ||
		strings.HasPrefix(uri, "https://") {
		return url.Parse(uri)
	}
	return s.playlistUrl.Parse(uri)
}

func (s *HLSStream) fetchPlaylist() (playlist *m3u8.MediaPlaylist, err error) {
	playlistUrl := s.playlistUrl.String()
	tryTimes(3, func() error {
		playlist, err = fetchPlaylist(s.ctx, playlistUrl)
		return err
	})
	return
}

func (s *HLSStream) fetchMapData(url string) (data []byte, err error) {
	tryTimes(3, func() error {
		data, err = fetchMapData(s.ctx, url)
		return err
	})
	return
}

func (s *HLSStream) chunkDownloader(ctx context.Context, extMapData []byte, chunk *myio.ChunkIO, i int, url string) func() error {
	return func() error {
		return tryTimes(3, func() error {
			return fetchSegment(ctx, url, chunk, i, extMapData)
		})
	}
}

func fetchPlaylist(ctx context.Context, url string) (*m3u8.MediaPlaylist, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	playlist, _, err := m3u8.DecodeFrom(resp.Body, false)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if playlist, ok := playlist.(*m3u8.MediaPlaylist); ok {
		return playlist, nil
	}
	return nil, ErrUnkownM3u8PlaylistType
}

func fetchMapData(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func fetchSegment(ctx context.Context, segmentUrl string, chunk *myio.ChunkIO, i int, extMapData []byte) error {
	request, err := http.NewRequestWithContext(ctx, "GET", segmentUrl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var dataReader io.Reader
	if len(extMapData) > 0 {
		dataReader = io.MultiReader(bytes.NewReader(extMapData), resp.Body)
	} else {
		dataReader = resp.Body
	}
	data, err := io.ReadAll(dataReader)
	if err != nil {
		return err
	}
	chunk.ZeroCopyFillChunk(i, data)
	return nil
}

func tryTimes(times int, fn func() error) error {
	var err error
	for i := 0; i < times; i++ {
		err = fn()
		if err == nil {
			return nil
		}
	}
	return err
}
