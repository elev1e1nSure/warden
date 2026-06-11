//go:build !windows

package main

import "os/exec"

func setupCmd(cmd *exec.Cmd) {}
