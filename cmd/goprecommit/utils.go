package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
)

type command struct {
	Bin string

	binPath once[string]
}

func (c *command) Command(ctx context.Context, args ...string) *exec.Cmd {
	binPath := c.binPath.Get(func() string {
		return mustFindBin(c.Bin)
	})

	return exec.CommandContext(ctx, binPath, args...)
}

func (c *command) handleOutput(output []byte, err error) (string, bool) {
	output = bytes.TrimSpace(output)

	if err != nil {
		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) && !exitErr.Success() {
			return string(output), false
		}

		Error(c.Bin, err)
		Exit(1)
	}

	return string(output), true
}

func (c *command) Output(ctx context.Context, args ...string) (string, bool) {
	cmd := c.Command(ctx, args...)

	cmd.Stderr = os.Stderr

	return c.handleOutput(cmd.Output())
}

func (c *command) MustOutput(ctx context.Context, args ...string) string {
	cmd := c.Command(ctx, args...)

	cmd.Stderr = os.Stderr

	output, ok := c.handleOutput(cmd.Output())
	if !ok {
		Error(c.Bin, "command unsuccessful")
		Exit(1)
	}

	return output
}

func (c *command) CombinedOutput(ctx context.Context, args ...string) (string, bool) {
	cmd := c.Command(ctx, args...)

	return c.handleOutput(cmd.CombinedOutput())
}

func env(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}

func mustFindBin(bin string) string {
	inPath, err := findBin(bin)
	if err != nil {
		Error(err)
		Exit(1)
	}

	return inPath
}

func findBin(bin string) (string, error) {
	return exec.LookPath(bin)
}

type once[V any] struct {
	vOnce sync.Once
	v     V
}

func (o *once[V]) Get(f func() V) V {
	o.vOnce.Do(func() {
		o.v = f()
	})

	return o.v
}
