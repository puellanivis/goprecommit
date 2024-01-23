package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GolintBin defines a structured interface to a `golint` binary.
type GolintBin struct {
	command
}

var golintCmd = GolintBin{
	command: command{
		Bin: "golint",
	},
}

var godocRegexp = regexp.MustCompile(" or be unexported$")

// Lint calls `golint` on the given pkg, and returns true if there are any non-empty lines printed.
//
// The first line printed is preceeded by an Error message of the package name.
// Lines from `golint` will have `trimPrefix` removed from the start of each line.
func (g *GolintBin) Lint(ctx context.Context, pkg, trimPrefix string, ignoreGodoc bool) bool {
	var issues int

	output, _ := g.CombinedOutput(ctx, pkg)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		if ignoreGodoc && godocRegexp.MatchString(line) {
			continue
		}

		if issues == 0 {
			switch pkg {
			case "":
				Error("golint", "<root package>")
			default:
				Error("golint", pkg)
			}
		}

		Warning("golint", strings.TrimPrefix(line, trimPrefix))
		issues++
	}

	return issues > 0
}

// GofmtBin provides a structured interface to a `gofmt` binary.
type GofmtBin struct {
	command
}

var gofmtCmd = GofmtBin{
	command: command{
		Bin: "gofmt",
	},
}

var goimportsCmd = GofmtBin{
	command: command{
		Bin: "goimports",
	},
}

// List returns all filenames from the given `filenames`,
// where that file is in need of formatting.
func (g *GofmtBin) List(ctx context.Context, filenames []string) []string {
	var issues []string

	output, _ := g.CombinedOutput(ctx, append([]string{"-l"}, filenames...)...)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		issues = append(issues, line)
	}

	sort.Strings(issues)

	return issues
}

// GoBin provides a structured interface to a `go` binary.
type GoBin struct {
	command

	ver once[*SemVer]
}

var goCmd = GoBin{
	command: command{
		Bin: env("GO", "go"),
	},
}

// Version returns the parsed SemVer returned by the binary.
func (g *GoBin) Version(ctx context.Context) *SemVer {
	return g.ver.Get(func() *SemVer {
		output, ok := goCmd.Output(ctx, "version")
		if !ok {
			Error("could not get go version")
			Exit(1)
		}

		if !strings.HasPrefix(output, "go version go") {
			Error("go version", "could not find version: ", output)
		}

		fields := strings.Fields(output)

		ver, err := ParseVersion(strings.TrimPrefix(fields[2], "go"))
		if err != nil {
			Error(err)
			Exit(1)
		}

		return ver
	})
}

func ensureGopathBinInPath() {
	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")

	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, path := range paths {
		if path == gopathBin {
			return
		}
	}

	newPath := strings.Join(append(paths, gopathBin), string(os.PathListSeparator))
	Warning("putting $GOPATH/bin into PATH", newPath)
	os.Setenv("PATH", newPath)
}

// Install verifies a given `bin` executable is installed,
// if it is not, then it will install that binary from `pkg` at `latest`.
//
// If anything fails to execute, it will print an Error, and exit.
func (g *GoBin) Install(ctx context.Context, bin, pkg string) {
	if _, err := findBin(bin); err == nil {
		return
	}

	ver := g.Version(ctx)
	if ver.Major == 1 && ver.Minor < 16 {
		output, ok := g.CombinedOutput(ctx, "get", "-u", pkg)
		if !ok {
			Error(output)
			Exit(1)
		}

		return
	}

	output, ok := g.CombinedOutput(ctx, "install", pkg+"@latest")
	if !ok {
		Error(output)
		Exit(1)
	}

	if _, err := findBin(bin); err != nil {
		Error("after installing binary", err)
		Exit(1)
	}
}

// ModTidy will run `go mod tidy` (or equivalent) in the current working directory.
func (g *GoBin) ModTidy(ctx context.Context) bool {
	var howtoTidy string

	ver := g.Version(ctx)

	switch {
	case ver.Major == 1 && ver.Minor < 11:
		return true

	case ver.Major == 1 && ver.Minor == 11 && ver.Details == "beta2":
		howtoTidy = "-sync"

	default:
		howtoTidy = "tidy"
	}

	prefix := "go mod " + howtoTidy

	output, ok := g.CombinedOutput(ctx, "mod", howtoTidy)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimPrefix(line, "go: ")

		if line == "" {
			continue
		}

		Hide(prefix, line)
	}

	return ok
}

type goList struct {
	format string
}

func (o *goList) build(packages []string) (ret []string) {
	ret = append(ret, "list")

	if o.format != "" {
		ret = append(ret, "-f", o.format)
	}

	return append(ret, packages...)
}

// GoListOption applies an optional feature onto a GoCmd.List command.
type GoListOption func(*goList)

// WithFormat applies a go template string to each package.
func WithFormat(template string) GoListOption {
	return func(o *goList) {
		o.format = template
	}
}

// List returns all packages returned by `go list` with the given arguments.
func (g *GoBin) List(ctx context.Context, packages any, opts ...GoListOption) []string {
	var args goList

	for _, opt := range opts {
		opt(&args)
	}

	var pkgs []string
	switch packages := packages.(type) {
	case string:
		pkgs = []string{packages}
	case []string:
		pkgs = packages
	default:
		panic("unsuppored type")
	}

	return strings.Split(g.MustOutput(ctx, args.build(pkgs)...), "\n")
}

type goTest struct {
	count *int
}

func (o *goTest) build(pkgs []string) (ret []string) {
	ret = append(ret, "test")

	if o.count != nil {
		ret = append(ret, fmt.Sprintf("-count=%d", *o.count))
	}

	return append(ret, pkgs...)
}

// GoTestOption applies an optional feature onto a GoCmd.Test command.
type GoTestOption func(o *goTest)

// WithCache will enable/disable cache during the GoBin.Test run.
func WithCache(flag bool) GoTestOption {
	if flag {
		return func(o *goTest) {
			o.count = nil
		}
	}

	count := 1

	return func(o *goTest) {
		o.count = &count
	}
}

// Test runs `go test` on the specified packages.
func (g *GoBin) Test(ctx context.Context, pkgs []string, opts ...GoTestOption) <-chan string {
	ch := make(chan string)

	var args goTest
	for _, opt := range opts {
		opt(&args)
	}

	go func() {
		defer close(ch)

		cmd := g.Command(ctx, args.build(pkgs)...)

		output, err := cmd.StdoutPipe()
		if err != nil {
			ch <- err.Error()
			return
		}

		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			ch <- err.Error()
			return
		}

		s := bufio.NewScanner(output)
		for s.Scan() {
			ch <- s.Text()
		}

		if err := s.Err(); err != nil {
			ch <- err.Error()
		}

		if err := cmd.Wait(); err != nil {
			ch <- err.Error()
		}
	}()

	return ch
}
