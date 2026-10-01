//go:build windows

package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/sys/windows"
)

func initConsole() func() {
	output, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		lipgloss.SetColorProfile(termenv.Ascii)
		return func() {}
	}
	var originalMode uint32
	if err := windows.GetConsoleMode(output, &originalMode); err != nil {
		lipgloss.SetColorProfile(termenv.Ascii)
		return func() {}
	}
	originalOutputCP, outputErr := windows.GetConsoleOutputCP()
	originalInputCP, inputErr := windows.GetConsoleCP()
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)
	mode := originalMode | windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(output, mode); err != nil {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	return func() {
		_ = windows.SetConsoleMode(output, originalMode)
		if outputErr == nil {
			_ = windows.SetConsoleOutputCP(originalOutputCP)
		}
		if inputErr == nil {
			_ = windows.SetConsoleCP(originalInputCP)
		}
	}
}
