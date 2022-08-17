package plex

import (
	"encoding/json"
	"errors"
	"io"
)

var Bilibili = new(bilibiliApi)

type bilibiliApi struct{}
type bilibiliApiResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		PlayUrlInfo *struct {
			PlayUrl *struct {
				Stream []*struct {
					ProtocolName string `json:"protocol_name"`
					Format       []*struct {
						FormatName string `json:"format_name"`
						Codec      []*struct {
							CodecName string `json:"codec_name"`
							BaseUrl   string `json:"base_url"`
							UrlInfo   []*struct {
								Host  string `json:"host"`
								Extra string `json:"extra"`
							} `json:"url_info"`
						} `json:"codec"`
					} `json:"format"`
				} `json:"stream"`
			} `json:"playurl"`
		} `json:"playurl_info"`
	} `json:"data"`
}

func (a *bilibiliApi) URL(id string) (string, string, error) {
	// 优先返回fmp4
	url, err := a.FMP4(id)
	if err == nil {
		return url, "fmp4", nil
	}

	// 其次返回ts格式
	url, err = a.TS(id)
	if err == nil {
		return url, "ts", nil
	}

	url, err = a.FLV(id)
	if err == nil {
		return url, "flv", nil
	}

	return "", "", errors.New("no stream found")
}

func (a *bilibiliApi) FLV(id string) (string, error) {
	return a.url(id, "http_stream", "flv")
}

func (a *bilibiliApi) TS(id string) (string, error) {
	return a.url(id, "http_hls", "ts")
}

func (a *bilibiliApi) FMP4(id string) (string, error) {
	return a.url(id, "http_hls", "fmp4")
}

func (a *bilibiliApi) url(id string, protocolName string, formatName string) (string, error) {
	api := "https://api.live.bilibili.com/xlive/web-room/v2/index/getRoomPlayInfo?room_id=" + id + "&codec=0,1&protocol=0,1"
	if formatName == "fmp4" {
		api += "&format=0,2"
	} else {
		api += "&format=0,1"
	}
	resp, err := httpClient.Get(api)
	if err != nil {
		return "", err
	}

	r := new(bilibiliApiResult)
	err = json.NewDecoder(resp.Body).Decode(r)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}

	if r.Code != 0 {
		return "", errors.New(r.Message)
	}

	if r.Data == nil ||
		r.Data.PlayUrlInfo == nil ||
		r.Data.PlayUrlInfo.PlayUrl == nil ||
		len(r.Data.PlayUrlInfo.PlayUrl.Stream) == 0 {
		return "", errors.New("no stream found")
	}

	streams := r.Data.PlayUrlInfo.PlayUrl.Stream
	targetStreamIndex := -1
	for i, stream := range streams {
		if stream.ProtocolName == protocolName {
			targetStreamIndex = i
			break
		}
	}
	if targetStreamIndex == -1 {
		return "", errors.New("no stream found")
	}

	stream := r.Data.PlayUrlInfo.PlayUrl.Stream[targetStreamIndex]
	if len(stream.Format) == 0 {
		return "", errors.New("no stream found")
	}

	format := stream.Format[0]
	if len(format.Codec) == 0 {
		return "", errors.New("no stream found")
	}

	codec := format.Codec[0]
	if len(codec.UrlInfo) == 0 {
		return "", errors.New("no stream found")
	}

	urlInfo := codec.UrlInfo[0]
	url := urlInfo.Host + codec.BaseUrl + urlInfo.Extra
	return url, nil
}
