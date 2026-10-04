//go:build !windows

package tui

func InitConsole() func() { return func() {} }
