package main

import (
	"fmt"
	"time"
)

// ANSI цветовые коды
const (
	reset = "\033[0m"
	bold  = "\033[1m"
	dim   = "\033[2m"

	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
	gray    = "\033[90m"
)

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func info(msg string) {
	fmt.Printf("%s[%s]%s %s[INFO]%s %s\n", gray, timestamp(), reset, cyan+bold, reset, msg)
}

func warn(msg string) {
	fmt.Printf("%s[%s]%s %s[WARN]%s %s\n", gray, timestamp(), reset, yellow+bold, reset, msg)
}

func logError(msg string) {
	fmt.Printf("%s[%s]%s %s[ERROR]%s %s\n", gray, timestamp(), reset, red+bold, reset, msg)
}

func success(msg string) {
	fmt.Printf("%s[%s]%s %s[OK]%s %s\n", gray, timestamp(), reset, green+bold, reset, msg)
}

func debug(msg string) {
	fmt.Printf("%s[%s]%s %s[DEBUG]%s %s\n", gray, timestamp(), reset, dim, reset, msg)
}

func request(method string, path string, status int) {
	statusColor := green
	if status >= 400 {
		statusColor = red
	}
	fmt.Printf("%s[%s]%s %s%s%s %s%s%s → %s%d%s\n",
		gray, timestamp(), reset,
		magenta+bold, method, reset,
		white, path, reset,
		statusColor+bold, status, reset)
}
