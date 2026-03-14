package termui

import (
	"bytes"
	"strings"
	"testing"
)

func TestActivityLineReusesSingleLine(t *testing.T) {
	var buf bytes.Buffer
	line := NewActivityLine(&buf)

	if err := line.Update("[1/2] checking example.net"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := line.Update("[2/2] checking missing.net"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := line.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	got := buf.String()
	if strings.Count(got, "\r") < 3 {
		t.Fatalf("activity output = %q, want carriage-return updates", got)
	}
	if !strings.Contains(got, "[1/2] checking example.net") || !strings.Contains(got, "[2/2] checking missing.net") {
		t.Fatalf("activity output = %q, want both updates", got)
	}
}

func TestActivityLineFinishWritesFinalLine(t *testing.T) {
	var buf bytes.Buffer
	line := NewActivityLine(&buf)

	if err := line.Update("[1/1] example.net emitted"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := line.Finish("checked 1 candidate(s), emitted 1"); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "checked 1 candidate(s), emitted 1\n") {
		t.Fatalf("activity output = %q, want final durable line", got)
	}
}
