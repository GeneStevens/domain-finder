package zonefile

import (
	"bufio"
	"io"
)

// Reader streams parsed zone records from an io.Reader.
type Reader struct {
	scanner *bufio.Scanner
}

// NewReader creates a streaming reader for zone file content.
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return &Reader{scanner: scanner}
}

// Next returns the next parsed record, skipping comments and blank lines.
func (r *Reader) Next() (Record, error) {
	for r.scanner.Scan() {
		if record, ok := ParseLine(r.scanner.Text()); ok {
			return record, nil
		}
	}
	if err := r.scanner.Err(); err != nil {
		return Record{}, err
	}
	return Record{}, io.EOF
}
