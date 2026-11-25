//go:build !linux

package main

import "errors"

func driveLinux(inputDoc, expectedDoc) error {
	return errors.New("platform mismatch: linux case on a non-linux host")
}
