package main

import (
	"context"
	"strings"
	"sync"
)

// GitBin defines a structured interface to a `git` binary.
type GitBin struct {
	command

	filesOnce sync.Once
	files     []string

	branchOnce sync.Once
	branch     string
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
	git.branchOnce.Do(func() {
		git.branch = git.MustOutput(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	})

	return git.branch
}

// Files returns all of the files checked in.
func (git *GitBin) Files(ctx context.Context) []string {
	git.filesOnce.Do(func() {
		git.files = strings.Split(git.MustOutput(ctx, "ls-files"), "\n")
	})

	return git.files
}

// CheckIgnore returns true if the given filename is ignored by git.
func (git *GitBin) CheckIgnore(ctx context.Context, filename string) bool {
	_, ok := git.CombinedOutput(ctx, "check-ignore", "-q", filename)
	return ok
}
