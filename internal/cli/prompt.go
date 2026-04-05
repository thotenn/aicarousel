package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// reader is the shared stdin reader. Using a single instance prevents buffered
// input from being lost between calls.
var reader = bufio.NewReader(os.Stdin)

// Ask prints label, reads a line from stdin and returns it trimmed.
func Ask(label string) (string, error) {
	fmt.Print(label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// AskNumber prints label, reads a line and tries to parse it as int.
// Returns (n, true, nil) on success, (0, false, nil) for non-numeric input,
// and (0, false, err) on I/O error.
func AskNumber(label string) (int, bool, error) {
	s, err := Ask(label)
	if err != nil {
		return 0, false, err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// Confirm prints a yes/no prompt. Accepts "s", "si", "y", "yes" as true.
// When defaultYes is true, an empty response is treated as yes.
func Confirm(label string, defaultYes bool) (bool, error) {
	hint := "s/N"
	if defaultYes {
		hint = "S/n"
	}
	s, err := Ask(fmt.Sprintf("%s [%s]: ", label, hint))
	if err != nil {
		return defaultYes, err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return defaultYes, nil
	}
	return s == "s" || s == "si" || s == "sí" || s == "y" || s == "yes", nil
}

// PressEnter displays msg (or a default prompt) and waits for Enter.
func PressEnter(msg string) {
	if msg == "" {
		msg = "Presiona Enter para continuar..."
	}
	fmt.Print("\n" + Dim + msg + Reset + " ")
	reader.ReadString('\n') //nolint:errcheck
}

// ClearScreen emits ANSI escape codes to clear the terminal.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}
