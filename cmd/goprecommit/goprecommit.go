package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	flag "github.com/puellanivis/breton/lib/gnuflag"
	"github.com/puellanivis/breton/lib/os/process"
)

// Version information ready for build-time injection.
var (
	Version    = "v0.0.0"
	Buildstamp = "dev"
)

// Flags are the flags available in this command.
var Flags = struct {
	Cache bool `desc:"use cached test results"`
	Lint  bool `desc:"use golint"`
	Color bool `desc:"use color"`

	NoGodoc bool `desc:"don't show godoc issues"`
}{
	Cache: true,
	Lint:  true,
	Color: true,
}

func init() {
	flag.Struct("", &Flags)
	flag.BoolFunc("nocache", "do not use cached test results", func() { Flags.Cache = false })
	flag.BoolFunc("nolint", "do not use golint", func() { Flags.Lint = false })
	flag.BoolFunc("nocolor", "do not use color", func() { Flags.Color = false })
}

const pathSep = string(os.PathSeparator)

// Exit is the prefered specific Exit function to be used by this command.
func Exit(code int) {
	process.Exit(code)
}

var goModules bool

var generatedCodeMarker = regexp.MustCompile(`^// Code generated by .* DO NOT EDIT\.$`)

func precommitCheckModule(ctx context.Context, goMod string) bool {
	Verbose("using go.mod", goMod)

	saveDir, err := os.Getwd()
	if err != nil {
		Error(goMod, "getwd:", err)
		return false
	}

	dir := filepath.Dir(goMod)
	if err := os.Chdir(dir); err != nil {
		Error(goMod, "chdir:", err)
		return false
	}
	defer func() {
		if err := os.Chdir(saveDir); err != nil {
			Error(goMod, "popdir:", err)
		}
	}()

	pwd, err := os.Getwd()
	if err != nil {
		Error(goMod, "getwd:", err)
		return false
	}

	modPath := strings.TrimPrefix(pwd, filepath.Join(os.Getenv("GOPATH"), "src")+pathSep)
	Verbose("found MOD_PATH", modPath)

	goModules := goModules
	if modPath != pwd || !testCanRead("go.mod") {
		Verbose("ignoring go modules…")
		goModules = false
	}

	testpkgPrefix := "." + pathSep

	var modBase string
	if goModules && testCanRead("go.mod") {
		modBase = getModuleName("go.mod")
		Verbose("found MOD_BASE", modBase)
	}

	Verbose("listing go files…")

	var goFiles []string
	for _, file := range gitCmd.Files(ctx) {
		if !strings.HasSuffix(file, ".go") {
			continue
		}

		if strings.HasPrefix(file, "vendor"+pathSep) {
			continue
		}

		if testEmpty(file) {
			continue
		}

		if testFileContains(file, generatedCodeMarker) {
			continue
		}

		goFiles = append(goFiles, file)
	}

	Verbose("found files", len(goFiles))

	Verbose("looking for subrepos…")

	subrepos := make(map[string]bool)
	WalkFS(".", func(name string, de os.DirEntry) {
		if !de.IsDir() {
			return
		}

		if de.Name() == ".git" {
			dirname := filepath.Dir(name)
			if dirname == "." {
				panic(WalkPrune)
			}

			Verbose("found subrepo", dirname)
			subrepos[dirname] = true
		}

		if strings.HasPrefix(de.Name(), ".") || de.Name() == "vendor" {
			panic(WalkPrune)
		}
	})

	var issues int

	if goModules {
		Verbose("go mod tidy…")

		if !goCmd.ModTidy(ctx) {
			return false
		}
	}

	Verbose("listing packages…")

	var gopkgs []string
	var testPkgs []string
	for _, pkg := range goCmd.List(ctx, "./...") {
		if strings.Contains(pkg, "/vendor/") {
			// older versions of Go could return vendored packages.
			continue
		}

		pkg = strings.ReplaceAll(pkg, "_"+pwd+pathSep, "")
		pkg = strings.ReplaceAll(pkg, "_"+pwd, ".")

		pkg = strings.ReplaceAll(pkg, modPath+pathSep, "")
		pkg = strings.ReplaceAll(pkg, modPath, ".")

		if modBase != "" {
			pkg = strings.ReplaceAll(pkg, modBase+pathSep, "")
			pkg = strings.ReplaceAll(pkg, modBase, ".")
		}

		if gitCmd.CheckIgnore(ctx, pkg) {
			Verbose("package is ignored in git", pkg)
			continue
		}

		if subrepos[pkg] {
			Verbose("package is in a subrepo:", pkg)
			continue
		}

		gopkgs = append(gopkgs, pkg)
		testPkgs = append(testPkgs, testpkgPrefix+pkg)
	}

	Verbose("found gopkgs", gopkgs)

	if len(goFiles) > 0 {
		Verbose("gofmt on files…")

		gofmt := gofmtCmd.List(ctx, goFiles)

		inGofmt := make(map[string]bool)
		for _, file := range gofmt {
			Error("gofmt", file)
			inGofmt[file] = true
			issues++
		}

		goimports := goimportsCmd.List(ctx, goFiles)
		for _, file := range goimports {
			if inGofmt[file] {
				continue
			}

			Error("goimports", file)
			issues++
		}
	}

	if Flags.Lint && len(gopkgs) > 0 {
		Verbose("golint on packages…")

		for _, pkg := range gopkgs {
			if pkg != modBase {
				pkg = strings.TrimPrefix(pkg, modBase)
			}
			if pkg == "/" {
				pkg = "."
			}
			pkg = strings.TrimPrefix(pkg, pathSep)

			if golintCmd.Lint(ctx, pkg, pwd+pathSep, Flags.NoGodoc) {
				issues++
			}
		}
	}

	if len(testPkgs) > 0 {
		Verbose("go test on packages…")

		if modBase != "" {
			// Exchange modBase for modPath, if it is set.
			modPath = modBase
		}

		for line := range goCmd.Test(ctx, testPkgs, WithCache(Flags.Cache)) {
			switch {
			case strings.HasPrefix(line, "go: "):
				// go messages should be shadowed.
				Hide("go test", line)

			case strings.HasPrefix(line, "ok") && strings.Contains(line, "(cached)"):
				if strings.Contains(line, "(cached)") {
					// Cached test results should be low-lighted
					Info("go test", line)
				} else {
					OK("go test", line)
				}

			case strings.HasPrefix(line, "PASS"):
				OK("go test", line)

			case line == "FAIL":
				// Ignore lines that just say "FAIL".

			case strings.HasPrefix(line, "FAIL"), strings.HasPrefix(line, "--- FAIL"):
				// Failures should be highlighted as errors.
				Error("go test", line)
				issues++

			case strings.HasPrefix(line, "panic:"):
				if strings.Contains(line, "[recovered]") {
					// recovered panics should be highlighted as warnings.
					Warning("go test", line)
				} else {
					// Unrecovered panics should be highlighted as errors.
					Error("go test", line)
				}
				issues++

			case strings.Contains(line, "cannot find package"):
				// Not being able to find a package should be highlighted as an error.
				Error("go test", line)
				issues++

			case strings.HasPrefix(line, "?") && strings.Contains(line, "[no test files]"):
				fields := strings.Fields(line)
				pkg := fields[1]

				// If pkg has a leading underscore, then replace "_${PWD}/" with "./".
				if try := strings.TrimPrefix(pkg, "_"+pwd); try != pkg {
					pkg = filepath.Join(".", try)
				}

				for _, pkgname := range goCmd.List(ctx, pkg, WithFormat("{{.Name}}")) {
					switch pkgname {
					case "main":
						// If a main package does not have tests, then it should be shadowed.
						Hide("go test", line)
					default:
						// Non-main packages with no test files should be lightly highlighted.
						Notice("go test", line)
					}
				}

			default:
				// Lines that we cannot recognize as anything else should be highlighted as warnings.
				Warning("go test", strings.ReplaceAll(line, pwd, "."))
				issues++
			}
		}
	}

	return issues == 0
}

func endsWithEOL(filename string) bool {
	f, err := os.Open(filename)
	if err != nil {
		Error("check eol", err)
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		Error("check eol", err)
		return false
	}
	if fi.Size() == 0 {
		// a completely empty file is zero lines of text, and thus valid.
		return true
	}

	if _, err := f.Seek(-1, os.SEEK_END); err != nil {
		Error("check eol", err)
		return false
	}

	var buf [1]byte

	if _, err := f.Read(buf[:]); err != nil {
		Error("check eol", err)
		return false
	}

	return buf[0] == '\n'
}

func main() {
	log.SetPrefix("goprecommit: ")
	log.SetFlags(0)

	ctx, finish := process.Init("goprecommit", Version, Buildstamp)
	defer finish()

	if !gitCmd.InRepo(ctx) {
		Verbose("not in git repo")
		return
	}

	Verbose("in git repo")

	version := goCmd.Version(ctx)
	Verbose("found go version", version)

	switch {
	case version.Major == 1 && version.Minor < 11:
	case version.Major == 1 && version.Minor == 11 && version.Details == "beta1":
	default:
		goModules = true
	}

	ensureGopathBinInPath()

	goCmd.Install(ctx, "golint", "golang.org/x/lint/golint")
	goCmd.Install(ctx, "goimports", "golang.org/x/tools/cmd/goimports")

	files := gitCmd.Files(ctx)

	var goMods []string
	for _, file := range files {
		select {
		case <-ctx.Done():
			Exit(1)
		default:
		}

		if strings.HasPrefix(file, "vendor/") || strings.Contains(file, "/vendor/") {
			continue
		}

		if filepath.Base(file) == "go.mod" {
			goMods = append(goMods, file)
		}
	}

	if len(goMods) > 0 {
		Verbose("found checked in go.mod files", len(goMods))
	} else {
		Warning("could not find any checked in go.mod files")
		goMods = append(goMods, filepath.Join(".", "go.mod"))
	}

	var blockCommit bool
	for _, goMod := range goMods {
		if !precommitCheckModule(ctx, goMod) {
			blockCommit = true
		}
	}

	branch := gitCmd.Branch(ctx)
	switch branch {
	case gitCmd.HeadBranch(ctx), "production", "staging":
		Error("branch name", "do not commit to ", branch)
		blockCommit = true
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			Exit(1)
		default:
		}

		if strings.HasSuffix(file, ".jar") {
			continue
		}

		if strings.Contains(file, ".") {
			if !endsWithEOL(file) {
				Error("file doesn't end with EOL", file)
				blockCommit = true
			}
		}
	}

	if blockCommit {
		Exit(1)
	}
}
