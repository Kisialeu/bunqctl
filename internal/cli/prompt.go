package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// readLine reads one line from the command input.
func (a *app) readLine(prompt string) (string, error) {
	fmt.Fprint(a.errOut, prompt)
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// confirm asks for want (or "y" when want is empty) and fails with
// errCancelled on any other answer. It refuses when input is not a terminal,
// so scripts cannot confirm by accident.
func (a *app) confirm(question, want string) error {
	if !a.interactive() {
		return fmt.Errorf("%w: confirmation needs a terminal", errCancelled)
	}
	hint := "[y/N]"
	if want != "" {
		hint = fmt.Sprintf("(type %s to confirm)", want)
	}
	answer, err := a.readLine(question + " " + hint + ": ")
	if err != nil {
		return err
	}
	if want == "" && strings.EqualFold(answer, "y") || want != "" && answer == want {
		return nil
	}
	return errCancelled
}

// readSecret reads a value without echo from the terminal.
func (a *app) readSecret(prompt string) (string, error) {
	if !a.interactive() {
		return "", errors.New("no terminal for a hidden prompt: use --api-key-stdin")
	}
	fmt.Fprint(a.errOut, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(a.errOut)
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func (a *app) interactive() bool {
	return a.in == os.Stdin && term.IsTerminal(int(os.Stdin.Fd()))
}
