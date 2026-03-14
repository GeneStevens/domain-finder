package termui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

type bubbleConsole struct {
	program *tea.Program
	model   interactiveModel
	w       io.Writer
	runErr  chan error
	started bool
	waited  sync.Once
	printed sync.Once
}

func newBubbleConsole(w io.Writer, zones []string, candidates []string, color, hideTaken, showPartials bool) *bubbleConsole {
	_ = hideTaken
	model := newInteractiveModel(zones, candidates, color, showPartials, nil)
	return &bubbleConsole{
		model: model,
		w:     w,
		program: tea.NewProgram(
			model,
			tea.WithInput(os.Stdin),
			tea.WithOutput(w),
		),
		runErr: make(chan error, 1),
	}
}

func (c *bubbleConsole) Start(total int, filter report.FilterMode) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.started = true
	go func() {
		finalModel, err := c.program.Run()
		if typed, ok := finalModel.(interactiveModel); ok {
			c.model = typed
		}
		c.runErr <- err
	}()
	c.program.Send(startMsg{total: total, filter: filter})
	return nil
}

func (c *bubbleConsole) UpdateActive(index, total int, candidate string) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(activeMsg{index: index, total: total, candidate: candidate})
	return nil
}

func (c *bubbleConsole) UpdateStatus(line string) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(statusMsg{line: line})
	return nil
}

func (c *bubbleConsole) EmitRow(result match.CandidateResult) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(rowMsg{result: result})
	return nil
}

func (c *bubbleConsole) ShouldEmitRow(result match.CandidateResult) bool {
	if c == nil {
		return true
	}
	return shouldEmitRow(c.model.showPartials, result)
}

func (c *bubbleConsole) ClearActive() error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(activeMsg{})
	return nil
}

func (c *bubbleConsole) Finish(summary report.Summary) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(finishMsg{summary: summary})
	if err := c.wait(); err != nil {
		return err
	}
	return c.printTranscript()
}

func (c *bubbleConsole) Note(line string) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(noteMsg{line: line})
	return nil
}

func (c *bubbleConsole) Close() error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Quit()
	if err := c.wait(); err != nil {
		return err
	}
	return c.printTranscript()
}

func (c *bubbleConsole) SetInterrupt(fn func()) {
	if c == nil {
		return
	}
	c.model.interrupt = fn
	if c.program != nil && c.started {
		c.program.Send(setInterruptMsg{fn: fn})
	}
}

func (c *bubbleConsole) wait() error {
	var err error
	c.waited.Do(func() {
		err = <-c.runErr
	})
	return err
}

func (c *bubbleConsole) printTranscript() error {
	var err error
	c.printed.Do(func() {
		transcript := c.model.Transcript()
		if strings.TrimSpace(transcript) == "" || c.w == nil {
			return
		}
		if _, err = fmt.Fprintln(c.w); err != nil {
			return
		}
		_, err = fmt.Fprintln(c.w, transcript)
	})
	return err
}
