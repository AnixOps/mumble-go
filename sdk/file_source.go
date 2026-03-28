package sdk

import (
	"context"
	"io"
	"os"
)

// FileSource streams raw PCM bytes from a local file.
// The file must already be 48kHz mono signed 16-bit little-endian PCM.
type FileSource struct {
	f *os.File
}

func NewFileSource(path string) (*FileSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &FileSource{f: f}, nil
}

func (s *FileSource) ReadPCM(ctx context.Context, dst []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	return s.f.Read(dst)
}

func (s *FileSource) Close() error {
	if s.f == nil {
		return nil
	}
	return s.f.Close()
}

var _ io.Closer = (*FileSource)(nil)
