package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// GitBin defines a structured interface to a `git` binary.
type GitBin struct {
	command

	mu    sync.Mutex
	files map[string][]string

	branch once[string]
	head   once[string]
}

var gitCmd = GitBin{
	command: command{
		Bin: env("GIT", "git"),
	},
}

// InRepo returns true if the current working directory is inside the git work tree.
func (git *GitBin) InRepo(ctx context.Context) bool {
	out, ok := git.CombinedOutput(ctx, "rev-parse", "--is-inside-work-tree")
	if !ok {
		return false
	}

	out = strings.TrimSpace(out)

	return out == "true"
}

// Branch returns the name of the current branch.
func (git *GitBin) Branch(ctx context.Context) string {
	return git.branch.Get(func() string {
		return git.MustOutput(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	})
}

// HeadBranch returns the name of the head branch.
func (git *GitBin) HeadBranch(ctx context.Context) string {
	return git.head.Get(func() string {
		head := git.MustOutput(ctx, "rev-parse", "--abbrev-ref", "refs/remotes/origin/HEAD")
		return strings.TrimPrefix(head, "origin/")
	})
}

// Files returns all of the files checked in.
func (git *GitBin) Files(ctx context.Context) []string {
	git.mu.Lock()
	defer git.mu.Unlock()

	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("could not get current working directory: %w", err))
	}

	files, ok := git.files[dir]
	if !ok {
		if git.files == nil {
			git.files = make(map[string][]string)
		}

		files = strings.Split(git.MustOutput(ctx, "ls-files"), "\n")
		git.files[dir] = files
	}

	return files
}

// CheckIgnore returns true if the given filename is ignored by git.
func (git *GitBin) CheckIgnore(ctx context.Context, filename string) bool {
	_, ok := git.CombinedOutput(ctx, "check-ignore", "-q", filename)
	return ok
}
