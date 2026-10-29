//go:build !linux

package main

// local test builds on Windows: no disk figure
func diskUsedPercent() float64 { return -1 }
