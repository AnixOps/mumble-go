package sdk

import (
	"context"
	"io"
	"os/exec"
)

// StreamingSource uses yt-dlp + ffmpeg to stream audio from arbitrary URLs
// (SoundCloud, YouTube, etc.) directly to PCM.
type StreamingSource struct {
	ffmpeg    *exec.Cmd
	ytDl      *exec.Cmd
	stdout    io.ReadCloser
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewStreamingSource creates a streaming source that decodes audio from a URL
// using yt-dlp to resolve the URL and ffmpeg to transcode to PCM.
func NewStreamingSource(ctx context.Context, url string) (*StreamingSource, error) {
	ctx, cancel := context.WithCancel(ctx)

	// First get the stream URL with yt-dlp
	ytDl := exec.CommandContext(ctx, resolveTool("yt-dlp"),
		"--no-playlist",
		"-o", "-",
		"-f", "bestaudio[ext=opus]/bestaudio/best",
		"--",
		url,
	)

	// ffmpeg reads from yt-dlp's stdout
	ffmpeg := exec.CommandContext(ctx, resolveTool("ffmpeg"),
		"-nostdin",
		"-loglevel", "info",
		"-i", "pipe:0",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"pipe:1",
	)

	// Connect yt-dlp stdout to ffmpeg stdin
	pipe, err := ytDl.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	ffmpeg.Stdin = pipe

	// ffmpeg stdout for reading PCM
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		pipe.Close()
		cancel()
		return nil, err
	}

	// Start yt-dlp first
	if err := ytDl.Start(); err != nil {
		stdout.Close()
		pipe.Close()
		cancel()
		return nil, err
	}

	// Start ffmpeg
	if err := ffmpeg.Start(); err != nil {
		ytDl.Process.Kill()
		ytDl.Wait()
		stdout.Close()
		pipe.Close()
		cancel()
		return nil, err
	}

	return &StreamingSource{
		ffmpeg:  ffmpeg,
		ytDl:    ytDl,
		stdout:  stdout,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (s *StreamingSource) ReadPCM(ctx context.Context, dst []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	default:
	}
	return s.stdout.Read(dst)
}

func (s *StreamingSource) Close() error {
	// Cancel context first to stop all processes
	s.cancel()

	// Close stdout
	s.stdout.Close()

	// Wait for processes
	if s.ffmpeg != nil {
		s.ffmpeg.Wait()
	}
	if s.ytDl != nil {
		s.ytDl.Wait()
	}
	return nil
}
