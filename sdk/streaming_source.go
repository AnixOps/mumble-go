package sdk

import (
	"context"
	"io"
	"os/exec"
)

// StreamingSource uses yt-dlp + ffmpeg to stream audio from arbitrary URLs
// (SoundCloud, YouTube, etc.) directly to PCM without intermediate files.
type StreamingSource struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

// NewStreamingSource creates a streaming source that decodes audio from a URL
// using yt-dlp to resolve the URL and ffmpeg to transcode to PCM.
func NewStreamingSource(ctx context.Context, url string) (*StreamingSource, error) {
	// Use yt-dlp with ffmpeg output to pipe directly to PCM
	// This handles authentication, HLS playlists, etc. automatically
	cmd := exec.CommandContext(ctx, resolveTool("yt-dlp"),
		"--no-playlist",
		"-o", "-",
		"-f", "bestaudio[ext=opus]/bestaudio/best",
		"--",
		url,
	)
	cmd.Stderr = nil // suppress yt-dlp output

	// Pipe yt-dlp output to ffmpeg for transcoding to PCM
	ffmpegCmd := exec.CommandContext(ctx, resolveTool("ffmpeg"),
		"-nostdin",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"pipe:1",
	)
	ffmpegCmd.Stdin, _ = cmd.StdoutPipe()

	stdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := ffmpegCmd.Start(); err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	// Clean up processes when done
	go func() {
		ffmpegCmd.Wait()
		cmd.Wait()
	}()

	return &StreamingSource{
		cmd:    ffmpegCmd,
		stdout: stdout,
	}, nil
}

func (s *StreamingSource) ReadPCM(ctx context.Context, dst []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	return s.stdout.Read(dst)
}

func (s *StreamingSource) Close() error {
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
	return nil
}
