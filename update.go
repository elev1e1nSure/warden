package tui

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	updateAPIURL  = "https://api.github.com/repos/elev1e1nSure/warden/releases/latest"
	updateZipHint = "windows-x64.zip"
)

type updateRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (m *model) runUpdate() tea.Cmd {
	return func() tea.Msg {
		err := prepareUpdate()
		return updateResultMsg{err: err}
	}
}

func prepareUpdate() error {
	if runtime.GOOS != "windows" {
		return errors.New("/update is only supported on Windows builds")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink: %w", err)
	}
	root := filepath.Dir(exePath)

	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}
	assetURL, assetName, err := pickWindowsAsset(rel)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "warden-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	zipPath := filepath.Join(tempDir, assetName)
	if err := downloadFile(assetURL, zipPath); err != nil {
		os.RemoveAll(tempDir)
		return err
	}
	if err := validateUpdateZip(zipPath); err != nil {
		os.RemoveAll(tempDir)
		return err
	}

	scriptPath := filepath.Join(tempDir, "apply-update.ps1")
	if err := os.WriteFile(scriptPath, []byte(updateScript), 0600); err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("write updater script: %w", err)
	}

	argsJSON, err := json.Marshal(os.Args[1:])
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("encode restart args: %w", err)
	}

	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-Root", root,
		"-ZipPath", zipPath,
		"-ParentPid", fmt.Sprintf("%d", os.Getpid()),
		"-ExePath", exePath,
		"-ArgsJson", string(argsJSON),
	)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("start updater: %w", err)
	}
	return nil
}

func fetchLatestRelease() (*updateRelease, error) {
	req, err := http.NewRequest(http.MethodGet, updateAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "warden-updater")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch latest release: %s", resp.Status)
	}

	var rel updateRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	return &rel, nil
}

func pickWindowsAsset(rel *updateRelease) (url string, name string, err error) {
	for _, asset := range rel.Assets {
		lower := strings.ToLower(asset.Name)
		if strings.HasPrefix(lower, "warden-") && strings.HasSuffix(lower, updateZipHint) && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL, asset.Name, nil
		}
	}
	return "", "", fmt.Errorf("latest release %s has no %s asset", rel.TagName, updateZipHint)
}

func downloadFile(url string, path string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "warden-updater")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create update zip: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write update zip: %w", err)
	}
	return nil
}

func validateUpdateZip(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open update zip: %w", err)
	}
	defer zr.Close()

	required := map[string]bool{
		"warden.exe": false,
	}
	for _, f := range zr.File {
		name := strings.ToLower(filepath.Base(f.Name))
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for name, found := range required {
		if !found {
			return fmt.Errorf("update zip is missing %s", name)
		}
	}
	return nil
}

const updateScript = `
param(
	[string]$Root,
	[string]$ZipPath,
	[int]$ParentPid,
	[string]$ExePath,
	[string]$ArgsJson
)

$ErrorActionPreference = "Stop"
$OutputEncoding = [Console]::OutputEncoding = [Text.Encoding]::UTF8

$tempDir = Split-Path -Parent $ZipPath
$extractDir = Join-Path $tempDir "extract"

try {
	Wait-Process -Id $ParentPid -ErrorAction SilentlyContinue
	Start-Sleep -Milliseconds 250

	if (Test-Path $extractDir) {
		Remove-Item -LiteralPath $extractDir -Recurse -Force
	}
	Expand-Archive -LiteralPath $ZipPath -DestinationPath $extractDir -Force

	$required = @("warden.exe")
	foreach ($name in $required) {
		$source = Get-ChildItem -LiteralPath $extractDir -Recurse -File -Filter $name | Select-Object -First 1
		if ($null -eq $source) {
			throw "update archive is missing $name"
		}
		Copy-Item -LiteralPath $source.FullName -Destination (Join-Path $Root $name) -Force
	}

	$argv = @()
	if ($ArgsJson -and $ArgsJson.Trim() -ne "") {
		$parsed = ConvertFrom-Json -InputObject $ArgsJson
		if ($null -ne $parsed) {
			$argv = [string[]]$parsed
		}
	}
	if ($argv.Count -gt 0) {
		Start-Process -FilePath $ExePath -ArgumentList $argv -WorkingDirectory $Root -NoNewWindow
	} else {
		Start-Process -FilePath $ExePath -WorkingDirectory $Root -NoNewWindow
	}
}
finally {
	Set-Location $env:TEMP
	Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
`
