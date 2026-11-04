package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// prompter asks the user for input on a terminal.
type prompter interface {
	// Interactive reports whether a person is there to answer.
	Interactive() bool
	// Line asks for a visible line of text.
	Line(prompt string) (string, error)
	// Secret asks for text that is not echoed.
	Secret(prompt string) (string, error)
}

// terminalPrompter prompts on stderr and reads from a terminal on stdin.
type terminalPrompter struct {
	in  *os.File
	out io.Writer
	buf *bufio.Reader
}

var _ prompter = (*terminalPrompter)(nil)

func newTerminalPrompter(in *os.File, out io.Writer) *terminalPrompter {
	return &terminalPrompter{in: in, out: out, buf: bufio.NewReader(in)}
}

func (p *terminalPrompter) Interactive() bool {
	return term.IsTerminal(int(p.in.Fd()))
}

func (p *terminalPrompter) Line(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.out, prompt); err != nil {
		return "", err
	}
	line, err := p.buf.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read answer: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (p *terminalPrompter) Secret(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.out, prompt); err != nil {
		return "", err
	}
	secret, err := term.ReadPassword(int(p.in.Fd()))
	if _, werr := fmt.Fprintln(p.out); werr != nil && err == nil {
		err = werr
	}
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	return string(secret), nil
}
