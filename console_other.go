//go:build !windows

package main

func initConsole() func() { return func() {} }
