//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// termCols is the width of the console on f, 0 when unknown.
func termCols(f *os.File) int {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(f.Fd()), &info); err != nil {
		return 0
	}
	return int(info.Window.Right - info.Window.Left + 1)
}
