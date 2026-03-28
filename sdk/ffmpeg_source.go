package sdk

import (
	"context"
	"io"
	"os"
	"os/exec"
)

// FFmpegSource uses ffmpeg to decode arbitrary audio files/streams into
// 48kHz mono signed 16-bit little-endian PCM.
type FFmpegSource struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

func NewFFmpegSource(input string) (*FFmpegSource, error) {
	cmd := exec.Command(resolveTool("ffmpeg"),
		"-nostdin",
		"-loglevel", "error",
		"-i", input,
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"pipe:1",
	)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &FFmpegSource{cmd: cmd, stdout: stdout}, nil
}

func (s *FFmpegSource) ReadPCM(ctx context.Context, dst []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	return s.stdout.Read(dst)
}

func (s *FFmpegSource) Close() error {
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	return nil
}
