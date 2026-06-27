package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── OCR PowerShell script (Windows.Media.Ocr) ───────────────────────────────
// Identical to the Python version — no extra deps, uses the OS OCR engine.

const ocrScript = `
$Path = $env:WARDEN_OCR_PATH
Add-Type -AssemblyName System.Runtime.WindowsRuntime | Out-Null
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
	$_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
	$_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1'
})[0]
function Await($op, $resultType) {
	$asTask = $asTaskGeneric.MakeGenericMethod($resultType)
	$task = $asTask.Invoke($null, @($op))
	$task.Wait(-1) | Out-Null
	$task.Result
}
[Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
[Windows.Storage.StorageFile, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
if ($null -eq $engine) { Write-Error 'OCR engine unavailable (no language pack installed)'; exit 1 }
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Path)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $result.Text
`

func runOCR(imagePath string) (string, error) {
	shell := shellExe()
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", ocrScript)
	cmd.Env = append(os.Environ(), "WARDEN_OCR_PATH="+imagePath)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(ee.Stderr))
			if stderr != "" {
				lines := strings.SplitN(stderr, "\n", 2)
				return "", fmt.Errorf("%s", lines[0])
			}
		}
		return "", fmt.Errorf("OCR failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// captureRegionToFile takes a screenshot (optionally cropped) and saves to temp.
// region: [x, y, w, h] in real screen pixels, or nil for full screen.
func captureRegionToFile(region []int) (string, error) {
	dir := screenshotDir()
	cleanupOldScreenshots(dir, 5*time.Minute)

	name := filepath.Join(dir, fmt.Sprintf("ocr_%s.png", time.Now().Format("20060102_150405_000")))

	if region != nil && len(region) == 4 {
		// Use PowerShell to capture and crop — avoids importing image processing in Go
		script := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing | Out-Null
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$g.Dispose()
$crop = $bmp.Clone((New-Object System.Drawing.Rectangle(%d,%d,%d,%d)), $bmp.PixelFormat)
$bmp.Dispose()
$crop.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$crop.Dispose()
`, region[0], region[1], region[2], region[3], strings.ReplaceAll(name, "'", "''"))
		shell := shellExe()
		cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("capture failed: %v", err)
		}
	} else {
		// Full screen capture via PowerShell GDI+
		script := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing | Out-Null
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$g.Dispose()
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
`, strings.ReplaceAll(name, "'", "''"))
		shell := shellExe()
		cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("capture failed: %v", err)
		}
	}
	return name, nil
}

// ── ImageLocateTool ──────────────────────────────────────────────────────────

// PowerShell GDI+ template matching script. Scans pixel-by-pixel.
const imageLocateScript = `
param($TemplatePath, $ScreenshotPath)
Add-Type -AssemblyName System.Drawing | Out-Null
$tmpl = [System.Drawing.Bitmap]::new($TemplatePath)
$screen = [System.Drawing.Bitmap]::new($ScreenshotPath)
$tw = $tmpl.Width; $th = $tmpl.Height
$sw = $screen.Width; $sh = $screen.Height
$found = $false
for ($sy = 0; $sy -le ($sh - $th); $sy++) {
	for ($sx = 0; $sx -le ($sw - $tw); $sx++) {
		$match = $true
		for ($ty = 0; $ty -lt $th -and $match; $ty += [Math]::Max(1, [int]($th/8))) {
			for ($tx = 0; $tx -lt $tw -and $match; $tx += [Math]::Max(1, [int]($tw/8))) {
				$sp = $screen.GetPixel($sx+$tx, $sy+$ty)
				$tp = $tmpl.GetPixel($tx, $ty)
				if ([Math]::Abs([int]$sp.R-[int]$tp.R) -gt 30 -or
					[Math]::Abs([int]$sp.G-[int]$tp.G) -gt 30 -or
					[Math]::Abs([int]$sp.B-[int]$tp.B) -gt 30) {
					$match = $false
				}
			}
		}
		if ($match) {
			# Verify with full pixel check on a sample
			$cx = $sx + [int]($tw/2); $cy = $sy + [int]($th/2)
			Write-Output "$cx,$cy,$tw,$th,$sx,$sy"
			$found = $true; break
		}
	}
	if ($found) { break }
}
if (-not $found) { Write-Output "NOTFOUND" }
$tmpl.Dispose(); $screen.Dispose()
`

type ImageLocateTool struct{}

func (t *ImageLocateTool) Name() string { return "image_locate" }

func (t *ImageLocateTool) Execute(args map[string]any) Result {
	path := strings.TrimSpace(getStr(args, "image"))
	if path == "" {
		return R("error: image path is required")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return R("error: image not found: " + path)
	}

	// Take a fresh screenshot for comparison
	screenshotPath, err := captureRegionToFile(nil)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer os.Remove(screenshotPath)

	shell := shellExe()
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", imageLocateScript,
		"-TemplatePath", path, "-ScreenshotPath", screenshotPath)
	out, err := cmd.Output()
	if err != nil {
		return R("error: image locate failed: " + err.Error())
	}

	result := strings.TrimSpace(string(out))
	if result == "NOTFOUND" || result == "" {
		return R("not found")
	}

	// Parse "cx,cy,tw,th,sx,sy"
	var cx, cy, tw, th, sx, sy int
	n, _ := fmt.Sscanf(result, "%d,%d,%d,%d,%d,%d", &cx, &cy, &tw, &th, &sx, &sy)
	if n < 4 {
		return R("not found")
	}
	mx, my := mapToModel(cx, cy)
	return R(fmt.Sprintf("found at (%d, %d) — size %dx%d, screen (%d, %d)", mx, my, tw, th, cx, cy))
}

// ── OcrTool ──────────────────────────────────────────────────────────────────

type OcrTool struct{}

func (t *OcrTool) Name() string { return "ocr" }

func (t *OcrTool) Execute(args map[string]any) Result {
	path := strings.TrimSpace(getStr(args, "image"))

	if path == "" {
		// Capture the screen, optionally a region
		var region []int
		if _, hasX := args["x"]; hasX {
			if _, hasY := args["y"]; hasY {
				if _, hasW := args["w"]; hasW {
					if _, hasH := args["h"]; hasH {
						rx, ry := mapToScreen(getInt(args, "x", 0), getInt(args, "y", 0))
						rw, rh := mapToScreen(getInt(args, "w", 0), getInt(args, "h", 0))
						region = []int{rx, ry, rw, rh}
					}
				}
			}
		}
		var err error
		path, err = captureRegionToFile(region)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer os.Remove(path)
	} else {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return R("error: image not found: " + path)
		}
	}

	text, err := runOCR(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if text == "" {
		return R("(no text recognized)")
	}
	return R(text)
}

// ── WaitForTool ──────────────────────────────────────────────────────────────

type WaitForTool struct{}

func (t *WaitForTool) Name() string { return "wait_for" }

func (t *WaitForTool) Execute(args map[string]any) Result {
	kind := strings.ToLower(strings.TrimSpace(getStr(args, "type")))
	target := strings.TrimSpace(getStr(args, "target"))
	if target == "" {
		return R("error: target is required")
	}

	timeout := getFloat(args, "timeout")
	if timeout <= 0 {
		timeout = 10.0
	}
	if timeout > 30.0 {
		timeout = 30.0
	}
	interval := getFloat(args, "interval")
	if interval <= 0 {
		interval = 0.5
	}
	if interval < 0.1 {
		interval = 0.1
	}

	if kind != "window" && kind != "text" && kind != "image" {
		return R("error: type must be window, text, or image")
	}

	start := time.Now()
	for {
		found, err := waitCheck(kind, target)
		if err != nil {
			return R("error: " + err.Error())
		}
		elapsed := time.Since(start).Seconds()
		if found {
			return R(fmt.Sprintf("found after %.1fs", elapsed))
		}
		if elapsed+interval >= timeout {
			return R(fmt.Sprintf("timeout: '%s' not found after %.0fs", target, timeout))
		}
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
	}
}

func waitCheck(kind, target string) (bool, error) {
	switch kind {
	case "window":
		windows, err := enumerateWindows()
		if err != nil {
			return false, err
		}
		needle := strings.ToLower(target)
		for _, w := range windows {
			if strings.Contains(strings.ToLower(w.title), needle) {
				return true, nil
			}
		}
		return false, nil
	case "image":
		screenshotPath, err := captureRegionToFile(nil)
		if err != nil {
			return false, err
		}
		defer os.Remove(screenshotPath)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return false, fmt.Errorf("template image not found: %s", target)
		}
		shell := shellExe()
		cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", imageLocateScript,
			"-TemplatePath", target, "-ScreenshotPath", screenshotPath)
		out, _ := cmd.Output()
		result := strings.TrimSpace(string(out))
		return result != "NOTFOUND" && result != "", nil
	case "text":
		path, err := captureRegionToFile(nil)
		if err != nil {
			return false, err
		}
		defer os.Remove(path)
		text, err := runOCR(path)
		if err != nil {
			return false, err
		}
		return strings.Contains(strings.ToLower(text), strings.ToLower(target)), nil
	}
	return false, nil
}
