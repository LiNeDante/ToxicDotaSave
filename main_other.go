//go:build !windows && !darwin && !dev

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, appName+" supports Windows and macOS only.")
	os.Exit(1)
}
