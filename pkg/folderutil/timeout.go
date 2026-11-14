package folderutil

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// errTimedOut is a command stopped by its timeout.
	errTimedOut = errors.New("timed out")
	// errInterrupted is a command stopped by Ctrl-C or SIGTERM.
	errInterrupted = errors.New("interrupted")
)

// interruptible ends on Ctrl-C, SIGTERM or SIGHUP (the terminal closed) when on says a command will run in
// a process group of its own: such a group never sees the terminal's Ctrl-C,
// so projekt takes it, and the context ending stops the group. Off, it
// catches nothing, so a caller that lives on — the MCP server — keeps its own
// signals.
func interruptible(on bool) (context.Context, context.CancelFunc) {
	if !on {
		return context.WithCancel(context.Background())
	}
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}

// limit bounds one call by timeout; zero or less means no limit.
func limit(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

// stopped says why ctx stopped a command, or returns err when it did not.
func stopped(ctx context.Context, err error) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return errTimedOut
	case ctx.Err() != nil:
		return errInterrupted
	}
	return err
}
