package tui

import (
	"fmt"
	"os"
	"time"
)

func info(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stdout, "[%s] [INFO] %s\n", ts, msg)
}

func logError(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] [ERROR] %s\n", ts, msg)
}

func success(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stdout, "[%s] [OK] %s\n", ts, msg)
}

func request(method string, path string, status int) {
	ts := time.Now().Format("15:04:05")
	statusColor := "\033[32m" // green
	if status < 200 || status >= 300 {
		statusColor = "\033[31m" // red
	}
	fmt.Fprintf(os.Stdout, "[%s] %s %s %s%d\033[0m\n", ts, method, path, statusColor, status)
}
