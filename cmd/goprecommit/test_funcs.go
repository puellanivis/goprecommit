package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
)

func testCanRead(filename string) bool {
	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()

	return true
}

func testEmpty(filename string) bool {
	fi, err := os.Stat(filename)
	if err != nil {
		return false
	}

	return fi.Size() == 0
}

func testFileContains(filename string, re *regexp.Regexp) bool {
	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if re.Match(s.Bytes()) {
			return true
		}
	}

	return false
}

func getModuleName(filename string) string {
	f, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()

		if bytes.HasPrefix(line, []byte("module ")) {
			fields := bytes.Fields(line)
			return string(fields[1])
		}
	}

	return ""
}

// WalkPrune can be used in a WalkFS to prevent descending into a subdirectory.
var WalkPrune struct{}

// WalkFS iterates through each directory entry in `dirname` and calls `fn` with the appropriate arguments.
//
// If `fn` calls `panic(WalkPrune)` and the given directory entry is a directory,
// then `WalkFS` will not descend into it.
func WalkFS(dirname string, fn func(fullname string, de os.DirEntry)) {
	direntries, err := os.ReadDir(dirname)
	if err != nil {
		Error(err)
		return
	}

	if dirname == "." {
		dirname = ""
	}

	for _, de := range direntries {
		fullname := filepath.Join(dirname, de.Name())

		func() {
			defer func() {
				if v := recover(); v != nil {
					if v == WalkPrune {
						return
					}

					panic(v)
				}
			}()

			fn(fullname, de)

			if de.IsDir() {
				WalkFS(fullname, fn)
			}
		}()
	}

}
