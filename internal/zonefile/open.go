package zonefile

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

var gzipMagic = [2]byte{0x1f, 0x8b}

// Stream is an opened zone file reader plus its underlying file handle.
type Stream struct {
	Reader io.Reader
	file   *os.File
	gzip   *gzip.Reader
}

// Close releases any open resources.
func (s *Stream) Close() error {
	var firstErr error
	if s.gzip != nil {
		if err := s.gzip.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Open opens a zone file and auto-detects gzip compression by inspecting the
// first two bytes. This intentionally ignores file extensions because CZDS
// files are sometimes gzip-compressed without a .gz suffix.
func Open(path string) (*Stream, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open zone file %q: %w", path, err)
	}

	reader := bufio.NewReader(file)
	isGzip, err := HasGzipMagic(reader)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect zone file %q: %w", path, err)
	}

	stream := &Stream{
		Reader: reader,
		file:   file,
	}

	if !isGzip {
		return stream, nil
	}

	gz, err := gzip.NewReader(reader)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("open gzip reader for %q: %w", path, err)
	}

	stream.Reader = gz
	stream.gzip = gz
	return stream, nil
}

// HasGzipMagic reports whether the next bytes in r match the gzip magic bytes.
// It uses Peek so callers can continue reading the same stream afterward.
func HasGzipMagic(r *bufio.Reader) (bool, error) {
	header, err := r.Peek(len(gzipMagic))
	if err != nil {
		if err == io.EOF || err == bufio.ErrBufferFull {
			return false, nil
		}
		return false, err
	}
	return header[0] == gzipMagic[0] && header[1] == gzipMagic[1], nil
}
