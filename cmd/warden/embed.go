//go:build !release

package main

var embeddedBackend []byte

func extractBackend(destDir string) (string, error) {
	return "", nil
}
