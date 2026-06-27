package tools

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type SystemInfoTool struct{}

func (t *SystemInfoTool) Name() string { return "system_info" }

func (t *SystemInfoTool) Execute(args map[string]any) Result {
	var lines []string
	lines = append(lines, fmt.Sprintf("os: %s %s", runtime.GOOS, runtime.GOARCH))
	host, _ := os.Hostname()
	lines = append(lines, "hostname: "+host)
	lines = append(lines, fmt.Sprintf("arch: %s", runtime.GOARCH))
	lines = append(lines, fmt.Sprintf("cpu: %d logical cores", runtime.NumCPU()))

	if ram := totalRAM(); ram > 0 {
		lines = append(lines, "ram: "+fmtBytes(float64(ram)))
	}
	if up := uptimeSeconds(); up > 0 {
		lines = append(lines, "uptime: "+fmtUptime(up))
	}

	if runtime.GOOS == "windows" {
		for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			drive := string(letter) + ":\\"
			if _, err := os.Stat(drive); err == nil {
				var free, total int64
				diskUsage(drive, &free, &total)
				used := total - free
				lines = append(lines, fmt.Sprintf("disk %s: %s / %s used", drive, fmtBytes(float64(used)), fmtBytes(float64(total))))
			}
		}
	} else {
		var free, total int64
		diskUsage("/", &free, &total)
		used := total - free
		lines = append(lines, fmt.Sprintf("disk /: %s / %s used", fmtBytes(float64(used)), fmtBytes(float64(total))))
	}

	return R(strings.Join(lines, "\n"))
}

// --- NotifyTool ---

type NotifyTool struct{}

func (t *NotifyTool) Name() string { return "notify" }

func (t *NotifyTool) Execute(args map[string]any) Result {
	if runtime.GOOS != "windows" {
		return R("error: notify is Windows-only")
	}
	message := strings.TrimSpace(getStr(args, "message"))
	if message == "" {
		return R("error: message is required")
	}
	title := strings.TrimSpace(getStr(args, "title"))
	if title == "" {
		title = "Warden"
	}
	script := `Add-Type -AssemblyName System.Windows.Forms | Out-Null
Add-Type -AssemblyName System.Drawing | Out-Null
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(5000, $env:WARDEN_NOTIFY_TITLE, $env:WARDEN_NOTIFY_MESSAGE, [System.Windows.Forms.ToolTipIcon]::Info)
Start-Sleep -Seconds 6
$n.Dispose()`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Env = append(os.Environ(),
		"WARDEN_NOTIFY_TITLE="+title,
		"WARDEN_NOTIFY_MESSAGE="+message,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return R("error: " + err.Error())
	}
	return R(fmt.Sprintf("notified: %s — %s", title, trunc(message, 60)))
}

// --- helpers ---

func fmtBytes(n float64) string {
	units := []string{"B", "KB", "MB", "GB"}
	for _, u := range units {
		if n < 1024 {
			return fmt.Sprintf("%.1f%s", n, u)
		}
		n /= 1024
	}
	return fmt.Sprintf("%.1fTB", n)
}

func totalRAM() int64 {
	if runtime.GOOS == "windows" {
		return winTotalRAM()
	}
	return readMemInfo()
}

func winTotalRAM() int64 {
	mod, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return 0
	}
	proc, err := mod.FindProc("GlobalMemoryStatusEx")
	if err != nil {
		return 0
	}
	type memStatus struct {
		length               uint32
		memoryLoad           uint32
		totalPhys            uint64
		availPhys            uint64
		totalPageFile        uint64
		availPageFile        uint64
		totalVirtual         uint64
		availVirtual         uint64
		availExtendedVirtual uint64
	}
	var ms memStatus
	ms.length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0
	}
	return int64(ms.totalPhys)
}

func readMemInfo() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var kb int64
				fmt.Sscanf(fields[1], "%d", &kb)
				return kb * 1024
			}
		}
	}
	return 0
}

func uptimeSeconds() float64 {
	if runtime.GOOS == "windows" {
		return winUptime()
	}
	return readUptime()
}

func winUptime() float64 {
	mod, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return 0
	}
	proc, err := mod.FindProc("GetTickCount64")
	if err != nil {
		return 0
	}
	ret, _, _ := proc.Call()
	return float64(ret) / 1000.0
}

func readUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	var secs float64
	fmt.Sscanf(fields[0], "%f", &secs)
	return secs
}

func fmtUptime(seconds float64) string {
	s := int(seconds)
	d := s / 86400
	s %= 86400
	h := s / 3600
	s %= 3600
	m := s / 60
	var parts []string
	if d > 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	if h > 0 || d > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	parts = append(parts, fmt.Sprintf("%dm", m))
	return strings.Join(parts, " ")
}

func diskUsage(path string, free, total *int64) {
	mod, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return
	}
	proc, err := mod.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return
	}
	ptr, _ := syscall.UTF16PtrFromString(path)
	proc.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(free)),
		uintptr(unsafe.Pointer(total)),
		0,
	)
}

func getStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt(m map[string]any, key string, defaultVal int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return defaultVal
}

func getFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func fmtAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
