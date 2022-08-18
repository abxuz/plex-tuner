package plex

import (
	"html"
	"io"
	"net/http"
	"path"

	"github.com/gorilla/websocket"
)

type (
	Object         = map[string]any
	Array          = []any
	ResponseWriter = http.ResponseWriter
	Request        = *http.Request
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (p *Plex) newHttpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", p.discover)
	mux.HandleFunc("/lineup_status.json", p.lineupStatus)
	mux.HandleFunc("/lineup.json", p.lineup)
	mux.HandleFunc("/stream/", p.stream)
	mux.HandleFunc("/", p.capability)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		disableCache(w)
		allowCORS(w)
		mux.ServeHTTP(w, r)
	})
}

func (p *Plex) discover(w ResponseWriter, r Request) {
	baseUrl := getBaseUrl(r)
	discoverData := Object{
		"FriendlyName":    "plex-tuner",
		"Manufacturer":    "Demon_H",
		"ModelNumber":     "plex-tuner",
		"FirmwareName":    "plex-tuner",
		"TunerCount":      p.config.TunerCount,
		"FirmwareVersion": Version,
		"DeviceID":        p.config.ID,
		"DeviceAuth":      "plex-tuner",
		"BaseURL":         baseUrl,
		"LineupURL":       baseUrl + "/lineup.json",
	}
	writeJson(w, discoverData)
}

func (p *Plex) capability(w ResponseWriter, r Request) {
	capData := `<root xmlns="urn:schemas-upnp-org:device-1-0">
    <specVersion>
        <major>1</major>
        <minor>0</minor>
    </specVersion>
    <URLBase>` + html.EscapeString(getBaseUrl(r)) + `</URLBase>
    <device>
        <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
        <friendlyName>plex-tuner</friendlyName>
        <manufacturer>b@doubi.fun</manufacturer>
        <modelName>plex-tuner</modelName>
        <modelNumber>plex-tuner</modelNumber>
        <serialNumber></serialNumber>
        <UDN>uuid:` + p.config.ID + `</UDN>
    </device>
</root>`

	header := w.Header()
	header.Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(capData))
}

func (p *Plex) lineupStatus(w ResponseWriter, r Request) {
	statusData := Object{
		"ScanInProgress": 0,
		"ScanPossible":   1,
		"Source":         "Cable",
		"SourceList":     Array{"Cable"},
	}
	writeJson(w, statusData)
}

func (p *Plex) lineup(w ResponseWriter, r Request) {
	if p.config.Channel == "" {
		internalServerError(w, "channel not configured")
		return
	}

	channels, err := getChannel(p.config.Channel)
	if err != nil {
		internalServerError(w, err.Error())
		return
	}

	baseUrl := getBaseUrl(r)
	lineupData := Array{}
	for _, channel := range channels {
		lineupData = append(lineupData, Object{
			"GuideNumber": channel.Id,
			"GuideName":   channel.Name,
			"URL":         baseUrl + "/stream/" + channel.Id,
		})
	}
	writeJson(w, lineupData)
}

func (p *Plex) stream(w ResponseWriter, r Request) {
	channels, err := getChannel(p.config.Channel)
	if err != nil {
		internalServerError(w, err.Error())
		return
	}

	id := path.Base(r.URL.Path)
	var target *Channel
	for _, channel := range channels {
		if channel.Id == id {
			target = channel
			break
		}
	}

	if target == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	disableKeepalive(w)

	switch target.Type {
	case "proxy", "hls":
		p.sharedStream(w, r, target)
	case "rtsp":
		p.unsharedStream(w, r, target)
	case "bilibili":
		p.sharedStream(w, r, target)
	case "redirect":
		http.Redirect(w, r, target.URL, http.StatusMovedPermanently)
	default:
		internalServerError(w, "unsupport channel type:"+target.Type)
	}
}

func (p *Plex) sharedStream(w ResponseWriter, r Request, channel *Channel) {
	reader, release, err := p.getChannelReader(channel)
	if err != nil {
		internalServerError(w, err.Error())
		return
	}
	defer release()
	p.warpReader(w, r, reader)
}

func (p *Plex) unsharedStream(w ResponseWriter, r Request, channel *Channel) {
	stream, err := p.createTVStream(channel)
	if err != nil {
		internalServerError(w, err.Error())
		return
	}
	defer stream.Close()
	if err := stream.Start(); err != nil {
		internalServerError(w, err.Error())
		return
	}
	p.warpReader(w, r, stream)
}

func (p *Plex) warpReader(w ResponseWriter, r Request, reader io.Reader) {
	if !isWebsocketUpgrade(r) {
		w.Header().Set("Content-Type", "video/mp4")
		io.Copy(w, reader)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		internalServerError(w, err.Error())
		return
	}
	defer conn.Close()

	buff := make([]byte, 100*1024)
	for {
		n, err := reader.Read(buff)
		if err != nil {
			break
		}
		err = conn.WriteMessage(websocket.BinaryMessage, buff[:n])
		if err != nil {
			break
		}
	}
}
