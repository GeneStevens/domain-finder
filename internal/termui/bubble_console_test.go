package termui

import (
	"bytes"
	"strings"
	"testing"
)

func TestBubbleConsolePrintTranscriptWritesDurableContent(t *testing.T) {
	var buf bytes.Buffer
	console := &bubbleConsole{
		w: &buf,
		model: interactiveModel{
			candWidth:   len("strongstem"),
			zoneWidth:   len("COM NET"),
			statusWidth: len("result"),
			results: []string{
				"strongstem  COM NET  all ✓",
			},
			notes: []string{
				"generation diagnostics",
				"  banned_prefix: 2",
			},
			summaryLine: "Done: checked 8 | emitted 1 | strong 1",
		},
	}

	if err := console.printTranscript(); err != nil {
		t.Fatalf("printTranscript() error = %v", err)
	}

	got := buf.String()
	for _, fragment := range []string{
		"strongstem  COM NET  all ✓",
		"generation diagnostics",
		"Done: checked 8 | emitted 1 | strong 1",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("transcript output missing %q:\n%s", fragment, got)
		}
	}
}
