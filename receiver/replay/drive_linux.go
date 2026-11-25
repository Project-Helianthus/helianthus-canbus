//go:build linux

package main

import "errors"

// errLinuxReplay is the linux result until a scratch overlay supplies scripted
// operands. This file does not open a host interface.
var errLinuxReplay = errors.New("linux replay requires the scratch overlay and does not open a host interface")

func driveLinux(inputDoc, expectedDoc) error {
	return errLinuxReplay
}
