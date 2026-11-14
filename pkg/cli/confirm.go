package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Confirm asks question on errOut and reads a yes or no from in. Only a
// terminal is asked: without one there is nobody to answer, so Confirm says
// how to keep the draft anyway and answers no.
func Confirm(in io.Reader, errOut io.Writer, question string) (bool, error) {
	f, ok := in.(*os.File)
	if !ok || (!isatty.IsTerminal(f.Fd()) && !isatty.IsCygwinTerminal(f.Fd())) {
		_, err := fmt.Fprintln(errOut, "not saved; run again with --yes to keep it")
		return false, err
	}
	if _, err := fmt.Fprint(errOut, question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	reply := strings.ToLower(strings.TrimSpace(line))
	return reply == "y" || reply == "yes", nil
}
