package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/valpere/kvach/internal/permission"
)

type stdioPermissionAsker struct {
	reader *bufio.Reader
	out    io.Writer
	mu     sync.Mutex
}

func newStdioPermissionAsker(in io.Reader, out io.Writer) permission.Asker {
	if in == nil || out == nil {
		return nil
	}
	return &stdioPermissionAsker{reader: bufio.NewReader(in), out: out}
}

func (a *stdioPermissionAsker) Ask(ctx context.Context, req permission.Request) (permission.Reply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for attempts := 0; attempts < 3; attempts++ {
		if err := ctx.Err(); err != nil {
			return permission.Reply{}, err
		}

		fmt.Fprintf(a.out, "\n[permission] %s\n", req.ToolName)
		fmt.Fprintf(a.out, "Description: %s\n", req.Description)
		fmt.Fprintf(a.out, "Risk: %s\n", req.Risk)
		fmt.Fprint(a.out, "Allow? [y] once / [a] always / [n] deny (default n): ")

		line, err := readLineWithContext(ctx, a.reader)
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(a.out)
				return permission.Reply{Decision: "deny", ToolName: req.ToolName, Pattern: "*"}, nil
			}
			return permission.Reply{}, err
		}

		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return permission.Reply{Decision: "allow_once", ToolName: req.ToolName, Pattern: "*"}, nil
		case "a", "always":
			return permission.Reply{Decision: "allow_always", ToolName: req.ToolName, Pattern: "*"}, nil
		case "", "n", "no":
			return permission.Reply{Decision: "deny", ToolName: req.ToolName, Pattern: "*"}, nil
		default:
			fmt.Fprintln(a.out, "Please answer y, a, or n.")
		}
	}

	return permission.Reply{Decision: "deny", ToolName: req.ToolName, Pattern: "*"}, nil
}

func readLineWithContext(ctx context.Context, reader *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if r.err == io.EOF {
			return r.line, io.EOF
		}
		return r.line, r.err
	}
}
