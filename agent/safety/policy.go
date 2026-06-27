package safety

import (
	"fmt"
	"os"
	"strings"
)

type SafetyDecision struct {
	Risk           string
	Reason         string
	Summary        string
	Details        []string
	NormalizedArgs map[string]any
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func applyMode(d SafetyDecision, toolName string, mode string) SafetyDecision {
	if mode == "auto" && d.Risk == "confirm" && toolName != "file_delete" && toolName != "delete" && toolName != "apply_patch" {
		return SafetyDecision{
			Risk: "safe", Reason: d.Reason, Summary: d.Summary,
			Details: d.Details, NormalizedArgs: d.NormalizedArgs,
		}
	}
	return d
}

func assessToolCall(toolName string, args map[string]any, cwd string, mode string) SafetyDecision {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	workspace := cwd

	norm := make(map[string]any, len(args))
	for k, v := range args {
		norm[k] = v
	}

	dec := func(risk, reason, summary string, details ...string) SafetyDecision {
		return applyMode(SafetyDecision{
			Risk: risk, Reason: reason, Summary: summary,
			Details: details, NormalizedArgs: norm,
		}, toolName, mode)
	}

	switch toolName {
	case "file_write", "write":
		path := getString(norm, "path")
		if isDangerousPath(path) {
			return dec("blocked", "dangerous path", "File path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		if !isPathWithinWorkspace(path, workspace) {
			return dec("blocked", "writes outside workspace", "Writing file outside workspace is blocked")
		}
		return dec("confirm", "modifies files", "Writing file inside workspace")

	case "file_delete", "delete":
		path := getString(norm, "path")
		recursive := getBool(norm, "recursive")
		if isDangerousPath(path) {
			return dec("blocked", "dangerous path", "File path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		if !isPathWithinWorkspace(path, workspace) {
			return dec("blocked", "deletes outside workspace", "Deleting file outside workspace is blocked")
		}
		var details []string
		if recursive {
			details = append(details, "recursive directory deletion")
		}
		return dec("confirm", "destructive file operation", "Deleting file inside workspace", details...)

	case "file_read", "read":
		path := getString(norm, "path")
		if isDangerousPath(path) {
			return dec("blocked", "dangerous path", "File path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		if !isPathWithinWorkspace(path, workspace) {
			return dec("confirm", "reads outside workspace", "Reading file outside workspace")
		}
		return dec("safe", "read-only", "Reading file")

	case "file_list", "list":
		path := getString(norm, "path")
		if path == "" {
			path = "."
		}
		if isDangerousPath(path) {
			return dec("blocked", "dangerous path", "Path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		if !isPathWithinWorkspace(path, workspace) {
			return dec("confirm", "lists outside workspace", "Listing directory outside workspace")
		}
		return dec("safe", "read-only", "Listing directory")

	case "todowrite":
		return dec("safe", "updates session todo state", "Updating todo list")

	case "skill":
		return dec("safe", "reads local skill files", "Loading skill",
			fmt.Sprintf("name: %s", getString(norm, "name")))

	case "bash", "powershell":
		command := getString(norm, "command")
		risk, reason, details := classify(command)
		summary := "Read-only shell command"
		if risk != "safe" {
			summary = capitalize(reason)
		}
		return applyMode(SafetyDecision{
			Risk: risk, Reason: reason, Summary: summary,
			Details: details, NormalizedArgs: norm,
		}, toolName, mode)

	case "clipboard":
		if strings.ToLower(getString(norm, "action")) == "read" {
			return dec("safe", "read-only", "Reading clipboard")
		}
		return dec("confirm", "modifies clipboard", "Writing to clipboard")

	case "screenshot":
		return dec("safe", "read-only", "Taking screenshot")

	case "mouse":
		action := strings.ToLower(getString(norm, "action"))
		if action == "" {
			action = "click"
		}
		if action == "move" {
			return dec("safe", "read-only pointer", "Moving cursor")
		}
		return dec("confirm", "simulates input", fmt.Sprintf("Mouse %s", action),
			"can interact with UI elements")

	case "keyboard":
		action := strings.ToLower(getString(norm, "action"))
		if action == "" {
			action = "type"
		}
		text := strings.ToLower(getString(norm, "text"))
		if action == "press" {
			dangerous := map[string]bool{
				"delete": true, "backspace": true, "alt+f4": true,
				"ctrl+w": true, "ctrl+shift+w": true,
			}
			for dk := range dangerous {
				if strings.Contains(text, dk) {
					return dec("confirm", "destructive key combination",
						fmt.Sprintf("Pressing %s", text),
						"can close windows or delete content")
				}
			}
		}
		return dec("confirm", "simulates input", fmt.Sprintf("Keyboard %s", action),
			"types or presses keys")

	case "browser_open":
		url := strings.ToLower(getString(norm, "url"))
		if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
			return dec("safe", "local URL", "Opening localhost URL",
				fmt.Sprintf("url: %s", url))
		}
		return dec("confirm", "opens external URL", "Opening external URL",
			fmt.Sprintf("url: %s", url))

	case "browser_read", "browser_screenshot", "youtube_search", "google_search":
		return dec("safe", "read-only", fmt.Sprintf("Using %s", toolName))

	case "apply_patch":
		return dec("confirm", "modifies files via patch", "Applying patch to files",
			"can create, modify, delete, or rename files")

	case "webfetch":
		url := strings.ToLower(getString(norm, "url"))
		if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") || strings.Contains(url, "::1") {
			return dec("safe", "read-only local", "Fetching local URL",
				fmt.Sprintf("url: %s", url))
		}
		return dec("safe", "read-only", fmt.Sprintf("Fetching %s", url))

	case "question":
		return dec("safe", "interactive", "Asking user")

	case "process_list":
		return dec("safe", "read-only", "Listing processes")

	case "process_kill":
		return dec("confirm", "terminates a process", "Killing process",
			"can disrupt the system or other applications")

	case "file_move", "file_copy":
		src := getString(norm, "src")
		dest := getString(norm, "dest")
		if isDangerousPath(src) || isDangerousPath(dest) {
			return dec("blocked", "dangerous path", "Path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		if !isPathWithinWorkspace(src, workspace) || !isPathWithinWorkspace(dest, workspace) {
			return dec("blocked", "path outside workspace",
				"file_move/file_copy outside workspace is blocked")
		}
		return dec("confirm", "mutates filesystem", fmt.Sprintf("%s inside workspace", toolName))

	case "archive":
		action := strings.ToLower(getString(norm, "action"))
		if action == "" {
			action = "list"
		}
		path := getString(norm, "path")
		if isDangerousPath(path) {
			return dec("blocked", "dangerous path", "Archive path is outside allowed scope",
				"UNC path, device path, or traversal detected")
		}
		switch action {
		case "list":
			return dec("safe", "read-only", "Listing archive")
		case "create":
			sources, _ := norm["sources"].([]any)
			if !isPathWithinWorkspace(path, workspace) {
				return dec("blocked", "path outside workspace", "Archive path is outside workspace")
			}
			for _, s := range sources {
				sStr := fmt.Sprint(s)
				if isDangerousPath(sStr) || !isPathWithinWorkspace(sStr, workspace) {
					return dec("blocked", "source outside workspace", "Archive source is outside workspace")
				}
			}
			return dec("confirm", "creates archive", "Creating archive")
		case "extract":
			dest := getString(norm, "dest")
			if dest == "" {
				dest = path
			}
			if isDangerousPath(dest) || !isPathWithinWorkspace(dest, workspace) {
				return dec("blocked", "path outside workspace", "Extract dest is outside workspace")
			}
			return dec("confirm", "extracts archive", "Extracting archive")
		default:
			return dec("confirm", "unknown archive action", fmt.Sprintf("archive %s", action),
				"action must be list, extract, or create")
		}

	case "window_list":
		return dec("safe", "read-only", "Listing windows")

	case "window_focus":
		return dec("confirm", "changes foreground window", "Focusing window",
			fmt.Sprintf("title: %s", getString(norm, "title")),
			fmt.Sprintf("hwnd: %s", getString(norm, "hwnd")))

	case "window_manage":
		action := strings.ToLower(getString(norm, "action"))
		return dec("confirm", "manipulates a window", fmt.Sprintf("Window %s", action),
			"can move, resize, minimize, maximize, or close windows")

	case "image_locate":
		return dec("safe", "read-only", "Locating image on screen")

	case "ocr":
		return dec("safe", "read-only", "Recognizing text on screen")

	case "wait_for":
		return dec("safe", "read-only polling",
			fmt.Sprintf("Waiting for %s", getString(norm, "type")),
			fmt.Sprintf("target: %s", getString(norm, "target")))

	case "system_info":
		return dec("safe", "read-only", "Reading system info")

	case "notify":
		return dec("safe", "shows a notification", "Sending desktop notification")

	case "memory":
		return dec("safe", "local notes store",
			fmt.Sprintf("memory %s", getString(norm, "action")))

	case "http_request":
		method := strings.ToUpper(getString(norm, "method"))
		if method == "" {
			method = "GET"
		}
		url := getString(norm, "url")
		switch method {
		case "GET", "HEAD", "OPTIONS":
			return dec("safe", "read-only request", fmt.Sprintf("%s %s", method, url))
		default:
			return dec("confirm", "sends a write request", fmt.Sprintf("%s %s", method, url),
				"can create or modify remote state")
		}

	case "browser_click":
		return dec("confirm", "interacts with a web page", "Clicking page element",
			fmt.Sprintf("selector: %s", getString(norm, "selector")))

	case "browser_fill":
		return dec("confirm", "interacts with a web page", "Filling page field",
			fmt.Sprintf("selector: %s", getString(norm, "selector")))

	default:
		return dec("confirm", "unknown tool", fmt.Sprintf("Unknown tool: %s", toolName),
			"no safety policy defined — requires confirmation")
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
