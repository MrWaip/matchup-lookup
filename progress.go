package main

import (
	"fmt"
	"os"
	"strings"
)

func showProgress(label string, done, total int) {
	if total == 0 {
		return
	}
	const width = 24
	filled := 0
	if total > 0 {
		filled = done * width / total
	}
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		if done == total || done%5 == 0 && done > 0 {
			fmt.Printf("[%s] %d/%d\n", label, done, total)
		}
		return
	}
	fmt.Printf("\r\x1b[2K[%s] [%s%s] %d/%d", label, strings.Repeat("█", filled), strings.Repeat("·", width-filled), done, total)
	if done == total {
		fmt.Println()
	}
}
