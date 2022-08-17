package tv

import (
	"io"
	"os/exec"
)

type FFMpegStream struct {
	cmd *exec.Cmd
	r   *io.PipeReader
	w   *io.PipeWriter
}

func NewFFMpegStream(ffmpeg string, url string) *FFMpegStream {
	s := &FFMpegStream{}
	s.r, s.w = io.Pipe()
	s.cmd = exec.Command(ffmpeg,
		"-i", url,
		"-c", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-g", "52",
		"-y", "pipe:1")
	s.cmd.Stdout = s.w
	return s
}

func (s *FFMpegStream) Start() error {
	return s.cmd.Start()
}

func (s *FFMpegStream) Close() error {
	s.w.Close()
	s.cmd.Process.Kill()
	return s.cmd.Wait()
}

func (s *FFMpegStream) Read(b []byte) (int, error) {
	return s.r.Read(b)
}
