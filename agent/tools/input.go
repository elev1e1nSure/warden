package tools

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
	"golang.org/x/sys/windows"
)

// ── coordinate space ─────────────────────────────────────────────────────────
// Screenshots are downscaled to cuMaxSide on their longest side before being
// shown to the model (keeps vision token cost sane). The model therefore points
// inside that downscaled image; mouse tools invert the scale to land on the
// real screen. tool_runner._encode_image uses the same constant — keep in sync.
const cuMaxSide = 1280

func scaleFactor(screenW, screenH int) float64 {
	longest := screenW
	if screenH > longest {
		longest = screenH
	}
	if longest <= cuMaxSide {
		return 1.0
	}
	return float64(cuMaxSide) / float64(longest)
}

func screenSize() (int, int) {
	n := screenshot.NumActiveDisplays()
	if n < 1 {
		return 0, 0
	}
	bounds := screenshot.GetDisplayBounds(0)
	return bounds.Dx(), bounds.Dy()
}

// mapToScreen maps model coords (downscaled-screenshot space) to real pixels.
func mapToScreen(x, y int) (int, int) {
	sw, sh := screenSize()
	scale := scaleFactor(sw, sh)
	if scale >= 1.0 {
		return x, y
	}
	mx := int(math.Round(float64(x) / scale))
	my := int(math.Round(float64(y) / scale))
	if mx < 0 {
		mx = 0
	}
	if mx >= sw {
		mx = sw - 1
	}
	if my < 0 {
		my = 0
	}
	if my >= sh {
		my = sh - 1
	}
	return mx, my
}

// mapToModel maps real screen pixels back to model coords.
func mapToModel(x, y int) (int, int) {
	sw, sh := screenSize()
	scale := scaleFactor(sw, sh)
	if scale >= 1.0 {
		return x, y
	}
	return int(math.Round(float64(x) * scale)), int(math.Round(float64(y) * scale))
}

// ── screenshot directory ─────────────────────────────────────────────────────

func screenshotDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.Getenv("TEMP")
	}
	if base == "" {
		home, _ := os.UserHomeDir()
		base = home
	}
	dir := filepath.Join(base, "warden", "temp_screenshots")
	os.MkdirAll(dir, 0o755)
	return dir
}

func cleanupOldScreenshots(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// ── key aliases ──────────────────────────────────────────────────────────────

var keyAliases = map[string]string{
	"control": "ctrl",
	"windows": "win",
	"super":   "win",
	"meta":    "win",
	"cmd":     "win",
	"command": "win",
	"option":  "alt",
	"return":  "enter",
	"escape":  "esc",
}

func normalizeKey(key string) string {
	k := strings.TrimSpace(strings.ToLower(key))
	if alias, ok := keyAliases[k]; ok {
		return alias
	}
	return k
}

// ── WinAPI SendInput ─────────────────────────────────────────────────────────

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procSendInput        = user32.NewProc("SendInput")
	procSetCursorPos     = user32.NewProc("SetCursorPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mousefAbsolute = 0x8000
	mousefMove     = 0x0001
	mousefLDown    = 0x0002
	mousefLUp      = 0x0004
	mousefRDown    = 0x0008
	mousefRUp      = 0x0010
	mousefWheel    = 0x0800

	keyfUnicode = 0x0004
	keyfKeyUp   = 0x0002
)

type mouseInput struct {
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type keybdInput struct {
	wVk       uint16
	wScan     uint16
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type inputUnion struct {
	typ  uint32
	data [40]byte // enough for largest union member
}

func sendMouseInput(dx, dy int32, data uint32, flags uint32) {
	mi := mouseInput{dx: dx, dy: dy, mouseData: data, flags: flags}
	var inp inputUnion
	inp.typ = inputMouse
	*(*mouseInput)(unsafe.Pointer(&inp.data[0])) = mi
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendKeybdInput(vk uint16, scan uint16, flags uint32) {
	ki := keybdInput{wVk: vk, wScan: scan, flags: flags}
	var inp inputUnion
	inp.typ = inputKeyboard
	*(*keybdInput)(unsafe.Pointer(&inp.data[0])) = ki
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func setCursorPos(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

// ── virtual key codes ────────────────────────────────────────────────────────

var vkMap = map[string]uint16{
	"backspace": 0x08, "tab": 0x09, "enter": 0x0D, "shift": 0x10,
	"ctrl": 0x11, "alt": 0x12, "pause": 0x13, "capslock": 0x14,
	"esc": 0x1B, "space": 0x20, "pageup": 0x21, "pagedown": 0x22,
	"end": 0x23, "home": 0x24, "left": 0x25, "up": 0x26,
	"right": 0x27, "down": 0x28, "printscreen": 0x2C, "insert": 0x2D,
	"delete": 0x2E, "win": 0x5B,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73,
	"f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
	"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	"numlock": 0x90, "scrolllock": 0x91,
}

func keyToVK(key string) (uint16, bool) {
	if vk, ok := vkMap[key]; ok {
		return vk, true
	}
	// Single character: use its uppercase ASCII as VK
	if len(key) == 1 {
		ch := key[0]
		if ch >= 'a' && ch <= 'z' {
			return uint16(ch - 32), true // VK_A..VK_Z
		}
		if ch >= '0' && ch <= '9' {
			return uint16(ch), true // VK_0..VK_9
		}
	}
	return 0, false
}

// ── ScreenshotTool ───────────────────────────────────────────────────────────

type ScreenshotTool struct{}

func (t *ScreenshotTool) Name() string { return "screenshot" }

func (t *ScreenshotTool) Execute(args map[string]any) Result {
	n := screenshot.NumActiveDisplays()
	if n < 1 {
		return R("error: no active display")
	}
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return R("error: " + err.Error())
	}

	dir := screenshotDir()
	cleanupOldScreenshots(dir, 5*time.Minute)
	name := filepath.Join(dir, fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405")))
	f, err := os.Create(name)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return R("error: " + err.Error())
	}

	sw, sh := bounds.Dx(), bounds.Dy()
	scale := scaleFactor(sw, sh)
	shownW := int(math.Round(float64(sw) * scale))
	shownH := int(math.Round(float64(sh) * scale))
	return R(fmt.Sprintf("saved: %s (screen %dx%d, shown %dx%d)", name, sw, sh, shownW, shownH))
}

// ── ClipboardTool ────────────────────────────────────────────────────────────

type ClipboardTool struct{}

func (t *ClipboardTool) Name() string { return "clipboard" }

func (t *ClipboardTool) Execute(args map[string]any) Result {
	action := getStr(args, "action")
	if action == "" {
		action = "read"
	}
	shell := shellExe()
	switch action {
	case "read":
		cmd := exec.Command(shell, "-NoProfile", "-Command",
			"[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; Get-Clipboard")
		out, err := cmd.Output()
		if err != nil {
			return R("error: " + err.Error())
		}
		text := strings.TrimSpace(string(out))
		if text == "" {
			return R("(empty)")
		}
		return R(text)
	case "write":
		text := getStr(args, "text")
		escaped := strings.ReplaceAll(text, "'", "''")
		cmd := exec.Command(shell, "-NoProfile", "-Command",
			fmt.Sprintf("Set-Clipboard -Value '%s'", escaped))
		if err := cmd.Run(); err != nil {
			return R("error: " + err.Error())
		}
		return R("copied to clipboard: " + trunc(text, 60))
	default:
		return R("error: action must be read or write")
	}
}

// clipboardPaste sets clipboard text and sends Ctrl+V via SendInput.
func clipboardPaste(text string) error {
	shell := shellExe()
	escaped := strings.ReplaceAll(text, "'", "''")
	cmd := exec.Command(shell, "-NoProfile", "-Command",
		fmt.Sprintf("Set-Clipboard -Value '%s'", escaped))
	if err := cmd.Run(); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	// Ctrl down
	sendKeybdInput(0x11, 0, 0)
	// V down
	sendKeybdInput(0x56, 0, 0)
	// V up
	sendKeybdInput(0x56, 0, keyfKeyUp)
	// Ctrl up
	sendKeybdInput(0x11, 0, keyfKeyUp)
	return nil
}

// ── MouseTool ────────────────────────────────────────────────────────────────

type MouseTool struct{}

func (t *MouseTool) Name() string { return "mouse" }

func (t *MouseTool) Execute(args map[string]any) Result {
	action := getStr(args, "action")
	if action == "" {
		action = "click"
	}
	x, y := mapToScreen(getInt(args, "x", 0), getInt(args, "y", 0))

	switch action {
	case "move":
		setCursorPos(x, y)
		time.Sleep(50 * time.Millisecond)
		return R(fmt.Sprintf("cursor → (%d, %d)", x, y))
	case "click":
		setCursorPos(x, y)
		time.Sleep(30 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefLDown)
		sendMouseInput(0, 0, 0, mousefLUp)
		return R(fmt.Sprintf("click (%d, %d)", x, y))
	case "right_click":
		setCursorPos(x, y)
		time.Sleep(30 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefRDown)
		sendMouseInput(0, 0, 0, mousefRUp)
		return R(fmt.Sprintf("right click (%d, %d)", x, y))
	case "double_click":
		setCursorPos(x, y)
		time.Sleep(30 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefLDown)
		sendMouseInput(0, 0, 0, mousefLUp)
		time.Sleep(50 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefLDown)
		sendMouseInput(0, 0, 0, mousefLUp)
		return R(fmt.Sprintf("double click (%d, %d)", x, y))
	case "scroll":
		amount := getInt(args, "amount", 3)
		setCursorPos(x, y)
		time.Sleep(30 * time.Millisecond)
		// WHEEL_DELTA = 120 per notch
		sendMouseInput(0, 0, uint32(int32(amount)*120), mousefWheel)
		return R(fmt.Sprintf("scroll %d @ (%d, %d)", amount, x, y))
	case "drag":
		x2, y2 := mapToScreen(getInt(args, "x2", 0), getInt(args, "y2", 0))
		setCursorPos(x, y)
		time.Sleep(50 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefLDown)
		time.Sleep(50 * time.Millisecond)
		setCursorPos(x2, y2)
		time.Sleep(100 * time.Millisecond)
		sendMouseInput(0, 0, 0, mousefLUp)
		return R(fmt.Sprintf("drag (%d, %d) → (%d, %d)", x, y, x2, y2))
	default:
		return R(fmt.Sprintf("error: unknown action '%s'", action))
	}
}

// ── KeyboardTool ─────────────────────────────────────────────────────────────

type KeyboardTool struct{}

func (t *KeyboardTool) Name() string { return "keyboard" }

func (t *KeyboardTool) Execute(args map[string]any) Result {
	action := getStr(args, "action")
	if action == "" {
		action = "type"
	}
	text := getStr(args, "text")

	switch action {
	case "type":
		if isASCII(text) {
			for _, ch := range text {
				sendKeybdInput(0, uint16(ch), keyfUnicode)
				sendKeybdInput(0, uint16(ch), keyfUnicode|keyfKeyUp)
				time.Sleep(20 * time.Millisecond)
			}
		} else {
			if err := clipboardPaste(text); err != nil {
				return R("error: " + err.Error())
			}
		}
		return R("typed: " + trunc(text, 60))
	case "press":
		keys := splitKeys(text)
		if len(keys) == 0 {
			return R("error: no key given")
		}
		// Press all modifiers down, then the final key, then release in reverse
		for _, k := range keys {
			vk, ok := keyToVK(normalizeKey(k))
			if !ok {
				return R(fmt.Sprintf("error: unknown key '%s'", k))
			}
			sendKeybdInput(vk, 0, 0)
		}
		for i := len(keys) - 1; i >= 0; i-- {
			vk, _ := keyToVK(normalizeKey(keys[i]))
			sendKeybdInput(vk, 0, keyfKeyUp)
		}
		return R("pressed: " + strings.Join(keys, "+"))
	default:
		return R("error: action must be type or press")
	}
}

func isASCII(s string) bool {
	for _, ch := range s {
		if ch > 127 {
			return false
		}
	}
	return true
}

func splitKeys(text string) []string {
	var keys []string
	for _, k := range strings.Split(text, "+") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}
