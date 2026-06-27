package tools

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ── WinAPI procedures ────────────────────────────────────────────────────────

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procMoveWindow               = user32.NewProc("MoveWindow")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

type windowInfo struct {
	pid   uint32
	title string
	hwnd  uintptr
	x, y  int
	w, h  int
}

// enumerateWindows returns all visible top-level windows with a title.
func enumerateWindows() ([]windowInfo, error) {
	var result []windowInfo

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		// Check visibility
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1 // continue
		}

		// Get title length
		length, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if length == 0 {
			return 1
		}

		// Get title text
		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
		title := windows.UTF16ToString(buf)
		if title == "" {
			return 1
		}

		// Get bounds
		var r rect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))

		// Get PID
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

		result = append(result, windowInfo{
			pid:   pid,
			title: title,
			hwnd:  hwnd,
			x:     int(r.Left),
			y:     int(r.Top),
			w:     int(r.Right - r.Left),
			h:     int(r.Bottom - r.Top),
		})
		return 1 // continue enumeration
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 && err != nil && err != syscall.Errno(0) {
		return nil, fmt.Errorf("EnumWindows failed: %v", err)
	}
	return result, nil
}

func matchWindow(wins []windowInfo, title string, hwnd int) *windowInfo {
	if hwnd != 0 {
		for i := range wins {
			if int(wins[i].hwnd) == hwnd {
				return &wins[i]
			}
		}
		return nil
	}
	if title != "" {
		needle := strings.ToLower(title)
		for i := range wins {
			if strings.Contains(strings.ToLower(wins[i].title), needle) {
				return &wins[i]
			}
		}
	}
	return nil
}

// ── WindowListTool ───────────────────────────────────────────────────────────

type WindowListTool struct{}

func (t *WindowListTool) Name() string { return "window_list" }

func (t *WindowListTool) Execute(args map[string]any) Result {
	filter := strings.ToLower(strings.TrimSpace(getStr(args, "filter")))
	wins, err := enumerateWindows()
	if err != nil {
		return R("error: " + err.Error())
	}
	if filter != "" {
		var filtered []windowInfo
		for _, w := range wins {
			if strings.Contains(strings.ToLower(w.title), filter) {
				filtered = append(filtered, w)
			}
		}
		wins = filtered
	}
	if len(wins) == 0 {
		if filter != "" {
			return R("no windows matching '" + filter + "'")
		}
		return R("no windows")
	}

	var b strings.Builder
	limit := len(wins)
	if limit > 100 {
		limit = 100
	}
	for i := 0; i < limit; i++ {
		w := wins[i]
		b.WriteString(fmt.Sprintf("%d  pid=%d  [%d,%d %dx%d]  %s\n",
			w.hwnd, w.pid, w.x, w.y, w.w, w.h, w.title))
	}
	if len(wins) > 100 {
		b.WriteString(fmt.Sprintf("... and %d more\n", len(wins)-100))
	}
	return R(strings.TrimRight(b.String(), "\n"))
}

// ── WindowFocusTool ──────────────────────────────────────────────────────────

type WindowFocusTool struct{}

func (t *WindowFocusTool) Name() string { return "window_focus" }

func (t *WindowFocusTool) Execute(args map[string]any) Result {
	title := strings.TrimSpace(getStr(args, "title"))
	hwnd := getInt(args, "hwnd", 0)
	if title == "" && hwnd == 0 {
		return R("error: give a title or hwnd")
	}

	wins, err := enumerateWindows()
	if err != nil {
		return R("error: " + err.Error())
	}
	win := matchWindow(wins, title, hwnd)
	if win == nil {
		return R("error: window not found")
	}

	// SW_RESTORE = 9
	procShowWindow.Call(win.hwnd, 9)
	procSetForegroundWindow.Call(win.hwnd)
	return R(fmt.Sprintf("focused: %s (hwnd=%d)", win.title, win.hwnd))
}

// ── WindowManageTool ─────────────────────────────────────────────────────────

const (
	swMinimize = 6
	swMaximize = 3
	swRestore  = 9
	wmClose    = 0x0010
)

type WindowManageTool struct{}

func (t *WindowManageTool) Name() string { return "window_manage" }

func (t *WindowManageTool) Execute(args map[string]any) Result {
	action := strings.ToLower(strings.TrimSpace(getStr(args, "action")))
	title := strings.TrimSpace(getStr(args, "title"))
	hwnd := getInt(args, "hwnd", 0)
	if title == "" && hwnd == 0 {
		return R("error: give a title or hwnd")
	}

	wins, err := enumerateWindows()
	if err != nil {
		return R("error: " + err.Error())
	}
	win := matchWindow(wins, title, hwnd)
	if win == nil {
		return R("error: window not found")
	}

	switch action {
	case "minimize":
		procShowWindow.Call(win.hwnd, swMinimize)
	case "maximize":
		procShowWindow.Call(win.hwnd, swMaximize)
	case "restore":
		procShowWindow.Call(win.hwnd, swRestore)
	case "close":
		procSendMessageW.Call(win.hwnd, wmClose, 0, 0)
	case "move", "resize":
		x := getInt(args, "x", win.x)
		y := getInt(args, "y", win.y)
		w := getInt(args, "w", win.w)
		h := getInt(args, "h", win.h)
		if action == "resize" && (w <= 0 || h <= 0) {
			return R("error: resize needs positive w and h")
		}
		procMoveWindow.Call(win.hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
	default:
		return R(fmt.Sprintf("error: unknown action '%s'", action))
	}

	return R(fmt.Sprintf("%s: %s (hwnd=%d)", action, win.title, win.hwnd))
}
