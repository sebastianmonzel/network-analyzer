//go:build !windows

package main

func handleServiceCommand(args []string) (bool, error) {
	return false, nil
}
