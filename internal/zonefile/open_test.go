package zonefile

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixturePath(parts ...string) string {
	all := append([]string{"..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func TestHasGzipMagic(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("example.net. 300 IN NS ns1.example.net.\n"))
		got, err := HasGzipMagic(reader)
		if err != nil {
			t.Fatalf("HasGzipMagic() error = %v", err)
		}
		if got {
			t.Fatal("HasGzipMagic() = true, want false")
		}
	})

	t.Run("gzip fixture without gz suffix", func(t *testing.T) {
		file, err := os.Open(fixturePath("small", "net.zone.slice"))
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer file.Close()

		reader := bufio.NewReader(file)
		got, err := HasGzipMagic(reader)
		if err != nil {
			t.Fatalf("HasGzipMagic() error = %v", err)
		}
		if !got {
			t.Fatal("HasGzipMagic() = false, want true")
		}
	})
}

func TestOpenPlain(t *testing.T) {
	stream, err := Open(fixturePath("small", "net.zone"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream.Reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(data), "example.net.") {
		t.Fatalf("plain data missing expected record: %q", string(data))
	}
}

func TestOpenGzipByContentWithoutExtension(t *testing.T) {
	stream, err := Open(fixturePath("small", "net.zone.slice"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream.Reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(data), "example.net.") {
		t.Fatalf("gzip data missing expected record: %q", string(data))
	}
}

func TestReaderStreamingPlainAndGzipMatch(t *testing.T) {
	plain, err := Open(fixturePath("small", "net.zone"))
	if err != nil {
		t.Fatalf("Open plain error = %v", err)
	}
	defer plain.Close()

	gz, err := Open(fixturePath("small", "net.zone.slice"))
	if err != nil {
		t.Fatalf("Open gzip error = %v", err)
	}
	defer gz.Close()

	plainDomains, err := collectDomains(NewReader(plain.Reader))
	if err != nil {
		t.Fatalf("collect plain error = %v", err)
	}
	gzipDomains, err := collectDomains(NewReader(gz.Reader))
	if err != nil {
		t.Fatalf("collect gzip error = %v", err)
	}

	if !reflect.DeepEqual(plainDomains, gzipDomains) {
		t.Fatalf("plain domains %v != gzip domains %v", plainDomains, gzipDomains)
	}
}

func collectDomains(reader *Reader) ([]string, error) {
	var domains []string
	for {
		record, err := reader.Next()
		if err == io.EOF {
			return domains, nil
		}
		if err != nil {
			return nil, err
		}
		domains = append(domains, record.Domain)
	}
}
