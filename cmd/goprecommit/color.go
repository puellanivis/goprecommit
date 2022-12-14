package main

import (
	"fmt"
)

func withColor(color, context string, a []any) string {
	var prefix, msg string = "", context

	if len(a) > 0 {
		if context != "" {
			prefix = context + ": "
		}
		msg = fmt.Sprint(a...)
	}

	if !Flags.Color {
		return prefix + msg
	}

	return prefix + "\x1b[0;" + color + ";1m" + msg + "\x1b[m"
}

// Verbose prints a verbose message.
func Verbose(context string, a ...any) {
	out.Verbose(withColor("37", context, a))
}

// Hide prints a message in dark-gray on black.
func Hide(context string, a ...any) {
	out.Info(withColor("90", context, a))
}

// OK prints a message in green on black.
func OK(context string, a ...any) {
	out.Info(withColor("92", context, a))
}

// Info prints a message in blue on black.
func Info(context string, a ...any) {
	out.Info(withColor("94", context, a))
}

// Notice prints a message in cyan on black.
func Notice(context string, a ...any) {
	out.Info(withColor("96", context, a))
}

// Warning prints a message in yellow on black.
func Warning(context string, a ...any) {
	out.Info(withColor("93", context, a))
}

// Error prints a message in red on black.
func Error(context any, a ...any) {
	out.Short(withColor("91", fmt.Sprint(context), a))
}
