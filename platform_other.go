//go:build !windows && !darwin

// Linux and others aren't supported targets; these stubs only let the core
// package build and its tests run in CI.
package main

import "errors"

func platformSteamDir() (string, error)       { return "", errors.New("unsupported platform") }
func platformActiveAccount() string           { return "" }
func platformDotaRunning() bool               { return false }
func platformReveal(p string, sel bool) error { return errors.New("unsupported platform") }
func platformOpenURL(u string) error          { return errors.New("unsupported platform") }
