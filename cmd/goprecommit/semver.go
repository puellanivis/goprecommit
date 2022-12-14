package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SemVer is a structured representation of Semantic Versioning.
type SemVer struct {
	Major, Minor, Patch int
	Details             string
}

var versionDetails = regexp.MustCompile("[a-z][a-z0-9]*$")

// ParseVersion takes an input string representation of a Semantic Versioning,
// and returns the structured representation of it.
func ParseVersion(input string) (*SemVer, error) {
	fields := strings.SplitN(strings.TrimPrefix(input, "v"), ".", 3)

	last := &fields[len(fields)-1]

	details := versionDetails.FindString(*last)
	*last = strings.TrimSuffix(*last, details)

	major, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, err
	}

	if len(fields) < 2 {
		return &SemVer{
			Major:   major,
			Details: details,
		}, nil
	}

	minor, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, err
	}

	if len(fields) < 3 {
		return &SemVer{
			Major:   major,
			Minor:   minor,
			Details: details,
		}, nil
	}

	patch, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, err
	}

	return &SemVer{
		Major:   major,
		Minor:   minor,
		Patch:   patch,
		Details: details,
	}, nil
}

func (v *SemVer) String() string {
	return fmt.Sprintf("v%d.%d.%d%s", v.Major, v.Minor, v.Patch, v.Details)
}
