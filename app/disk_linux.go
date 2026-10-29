//go:build linux

package main

import "syscall"

func diskUsedPercent() float64 {
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) != nil || st.Blocks == 0 {
		return -1
	}
	return float64(st.Blocks-st.Bfree) * 100 / float64(st.Blocks)
}
