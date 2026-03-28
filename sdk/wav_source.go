package sdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WAVSource streams PCM from a minimal WAV file.
// Supported format: PCM, mono, 48kHz, 16-bit little-endian.
type WAVSource struct {
	f         *os.File
	dataStart int64
	dataLen   uint32
	read      uint32
}

func NewWAVSource(path string) (*WAVSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	head := make([]byte, 44)
	if _, err := io.ReadFull(f, head); err != nil {
		f.Close()
		return nil, err
	}
	if string(head[0:4]) != "RIFF" || string(head[8:12]) != "WAVE" {
		f.Close()
		return nil, fmt.Errorf("sdk: unsupported wav header")
	}
	audioFormat := binary.LittleEndian.Uint16(head[20:22])
	channels := binary.LittleEndian.Uint16(head[22:24])
	sampleRate := binary.LittleEndian.Uint32(head[24:28])
	bitsPerSample := binary.LittleEndian.Uint16(head[34:36])
	if audioFormat != 1 || channels != 1 || sampleRate != 48000 || bitsPerSample != 16 {
		f.Close()
		return nil, fmt.Errorf("sdk: wav must be PCM mono 48kHz 16-bit")
	}
	dataLen := binary.LittleEndian.Uint32(head[40:44])
	return &WAVSource{f: f, dataStart: 44, dataLen: dataLen}, nil
}

func (s *WAVSource) ReadPCM(_ context.Context, dst []byte) (int, error) {
	if s.read >= s.dataLen {
		return 0, io.EOF
	}
	remaining := int(s.dataLen - s.read)
	if len(dst) > remaining {
		dst = dst[:remaining]
	}
	n, err := s.f.Read(dst)
	s.read += uint32(n)
	return n, err
}

func (s *WAVSource) Close() error {
	if s.f == nil {
		return nil
	}
	return s.f.Close()
}
