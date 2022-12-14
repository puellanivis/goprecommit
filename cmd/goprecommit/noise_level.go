package main

import (
	"log"

	flag "github.com/puellanivis/breton/lib/gnuflag"
)

var out = NoiseLevelNormal

func init() {
	flag.BoolFunc("verbose", "sverbose output", func() { out = NoiseLevelVerbose })
	flag.BoolFunc("short", "short output", func() { out = NoiseLevelShort })
	flag.BoolFunc("quiet", "no output", func() { out = NoiseLevelQuiet })
}

// NoiseLevel defines an amount of noisiness that a command should use when printing information.
type NoiseLevel int

// NoiseLevels that are defined:
const (
	NoiseLevelQuiet NoiseLevel = iota
	NoiseLevelShort
	NoiseLevelNormal
	NoiseLevelVerbose
)

// Verbose prints the given message only if NoiseLevel is Verbose or above.
func (l NoiseLevel) Verbose(msg string) {
	if l >= NoiseLevelVerbose {
		log.Print(msg)
	}
}

// Info prints the given message only if NoiseLevel is Normal or above.
func (l NoiseLevel) Info(msg string) {
	if l >= NoiseLevelNormal {
		log.Print(msg)
	}
}

// Short prints the given message only if NoiseLevel is Short or above.
func (l NoiseLevel) Short(msg string) {
	if l >= NoiseLevelShort {
		log.Print(msg)
	}
}
