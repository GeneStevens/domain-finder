package termui

import (
	"io"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

type bubbleConsole struct {
	program *tea.Program
	model   interactiveModel
	runErr  chan error
	once    sync.Once
}

func newBubbleConsole(w io.Writer, zones []string, candidates []string, color, hideTaken, showPartials bool) *bubbleConsole {
	_ = hideTaken
	model := newInteractiveModel(zones, candidates, color, showPartials, nil)
	return &bubbleConsole{
		model: model,
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
	go func() {
		_, err := c.program.Run()
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
	return c.wait()
}

func (c *bubbleConsole) Note(line string) error {
	if c == nil || c.program == nil {
		return nil
	}
	c.program.Send(noteMsg{line: line})
	return nil
}

func (c *bubbleConsole) wait() error {
	var err error
	c.once.Do(func() {
		err = <-c.runErr
	})
	return err
}
