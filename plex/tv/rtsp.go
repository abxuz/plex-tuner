package tv

type RTSPStream struct {
	url string
}

func NewRTSPStream(url string) *RTSPStream {
	return &RTSPStream{url: url}
}
