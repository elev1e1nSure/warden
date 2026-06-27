package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// --- FileReadTool ---

type FileReadTool struct{}

func (t *FileReadTool) Name() string { return "file_read" }

func (t *FileReadTool) Execute(args map[string]any) Result {
	path := getStr(args, "path")
	offset := getInt(args, "offset", 1)
	if offset < 1 {
		offset = 1
	}
	limit := getInt(args, "limit", 0)

	fi, err := os.Stat(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if fi.IsDir() {
		return R("error: is a directory, not a file: " + path)
	}
	if fi.Size() > 50*1024*1024 {
		return R("error: file too large (>50MB) — use offset/limit or grep: " + path)
	}

	f, err := os.Open(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer f.Close()

	var rawLines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		rawLines = append(rawLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return R("error: " + err.Error())
	}

	start := offset - 1
	end := len(rawLines)
	if limit > 0 {
		end = start + limit
	}
	if start >= len(rawLines) {
		return R("")
	}
	if end > len(rawLines) {
		end = len(rawLines)
	}
	slice := rawLines[start:end]

	var b strings.Builder
	for i, line := range slice {
		lineNum := start + i + 1
		if len(line) > 2000 {
			line = line[:2000] + "…"
		}
		b.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))
	}
	result := b.String()
	if len(result) > 8000 {
		result = result[:8000] + "\n...(truncated)"
	}
	return R(result)
}

// --- GlobTool ---

type GlobTool struct{}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Execute(args map[string]any) Result {
	pattern := getStr(args, "pattern")
	basePath := getStr(args, "path")
	if basePath == "" {
		basePath = "."
	}
	base, err := filepath.Abs(basePath)
	if err != nil {
		return R("error: " + err.Error())
	}

	// Fast path: use filepath.Glob if no **, otherwise walk
	var matches []string
	if strings.Contains(pattern, "**") {
		err = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			matched, err := filepath.Match(pattern, p)
			if err != nil {
				return nil
			}
			// Also try matching relative to base
			if !matched {
				rel, _ := filepath.Rel(base, p)
				matched, _ = filepath.Match(pattern, rel)
				if !matched {
					matched, _ = filepath.Match(pattern, strings.ReplaceAll(rel, "\\", "/"))
				}
			}
			if matched {
				matches = append(matches, p)
			}
			return nil
		})
	} else {
		matches, err = filepath.Glob(filepath.Join(base, pattern))
	}
	if err != nil {
		return R("error: " + err.Error())
	}

	// Sort by modification time, newest first
	sort.Slice(matches, func(i, j int) bool {
		fi, err1 := os.Stat(matches[i])
		fj, err2 := os.Stat(matches[j])
		if err1 != nil || err2 != nil {
			return matches[i] < matches[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})

	if len(matches) == 0 {
		return R("(no matches)")
	}

	var b strings.Builder
	count := 0
	for _, p := range matches {
		if count >= 200 {
			b.WriteString(fmt.Sprintf("\n... and %d more", len(matches)-200))
			break
		}
		rel, _ := filepath.Rel(base, p)
		rel = strings.ReplaceAll(rel, "\\", "/")
		if count > 0 {
			b.WriteString("\n")
		}
		b.WriteString(rel)
		count++
	}
	return R(b.String())
}

// --- GrepTool ---

type GrepTool struct{}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Execute(args map[string]any) Result {
	pattern := getStr(args, "pattern")
	path := getStr(args, "path")
	if path == "" {
		path = "."
	}
	globFilter := getStr(args, "glob")
	nocase, _ := args["case_insensitive"].(bool)

	// Try rg first
	if rgPath, err := exec.LookPath("rg"); err == nil {
		cmdArgs := []string{"--line-number", "--no-heading", "--color=never", "--max-count=100"}
		if nocase {
			cmdArgs = append(cmdArgs, "-i")
		}
		if globFilter != "" {
			cmdArgs = append(cmdArgs, "--glob", globFilter)
		}
		cmdArgs = append(cmdArgs, pattern, path)
		cmd := exec.Command(rgPath, cmdArgs...)
		out, err := cmd.Output()
		if err == nil {
			output := strings.TrimSpace(string(out))
			if output == "" {
				return R("(no matches)")
			}
			lines := strings.Split(output, "\n")
			if len(lines) > 100 {
				return R(strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... and %d more", len(lines)-100))
			}
			return R(output)
		}
	}

	// Go fallback
	rePattern := pattern
	if nocase {
		rePattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(rePattern)
	if err != nil {
		return R("error: " + err.Error())
	}

	base, err := filepath.Abs(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	fi, err := os.Stat(base)
	if err != nil {
		return R("error: " + err.Error())
	}

	var files []string
	if !fi.IsDir() {
		files = []string{base}
	} else {
		walkFn := func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if globFilter != "" {
				matched, _ := filepath.Match(globFilter, info.Name())
				if !matched {
					return nil
				}
			}
			files = append(files, p)
			return nil
		}
		filepath.Walk(base, walkFn)
	}
	sort.Strings(files)

	var results []string
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil || fi.IsDir() || fi.Size() > 10*1024*1024 {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(base, f)
		if fi.IsDir() {
			rel = f
		}
		rel = strings.ReplaceAll(rel, "\\", "/")
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimRight(line, "\r")))
				if len(results) >= 100 {
					break
				}
			}
		}
		if len(results) >= 100 {
			break
		}
	}
	if len(results) == 0 {
		return R("(no matches)")
	}
	return R(strings.Join(results, "\n"))
}

// --- EditTool ---

type EditTool struct{}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Execute(args map[string]any) Result {
	path := getStr(args, "path")
	oldStr := getStr(args, "old_string")
	newStr := getStr(args, "new_string")
	if oldStr == "" {
		return R("error: old_string is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	content := string(data)
	count := strings.Count(content, oldStr)

	if count == 0 {
		normContent := strings.ReplaceAll(content, "\r\n", "\n")
		normOld := strings.ReplaceAll(oldStr, "\r\n", "\n")
		if strings.Count(normContent, normOld) == 1 {
			newContent := strings.Replace(normContent, normOld, strings.ReplaceAll(newStr, "\r\n", "\n"), 1)
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return R("error: " + err.Error())
			}
			stats := diffStats(oldStr, newStr)
			label := fmt.Sprintf("edited %s", path)
			if stats != "" {
				label += "  " + stats
			}
			return RD(label, diffFull(oldStr, newStr, path))
		}
		return R("error: old_string not found in " + path)
	}
	if count > 1 {
		return R(fmt.Sprintf("error: old_string matches %d times — make it more specific", count))
	}
	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return R("error: " + err.Error())
	}
	stats := diffStats(oldStr, newStr)
	label := fmt.Sprintf("edited %s", path)
	if stats != "" {
		label += "  " + stats
	}
	return RD(label, diffFull(oldStr, newStr, path))
}

// --- FileWriteTool ---

type FileWriteTool struct{}

func (t *FileWriteTool) Name() string { return "file_write" }

func (t *FileWriteTool) Execute(args map[string]any) Result {
	path := getStr(args, "path")
	content := getStr(args, "content")
	absPath, err := filepath.Abs(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if d := filepath.Dir(absPath); d != "" {
		os.MkdirAll(d, 0755)
	}
	var oldContent string
	if data, err := os.ReadFile(path); err == nil {
		oldContent = string(data)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return R("error: " + err.Error())
	}
	stats := diffStats(oldContent, content)
	label := fmt.Sprintf("wrote %s", path)
	if stats != "" {
		label += "  " + stats
		return RD(label, diffFull(oldContent, content, path))
	}
	return R(label)
}

// --- FileDeleteTool ---

type FileDeleteTool struct{}

func (t *FileDeleteTool) Name() string { return "file_delete" }

func (t *FileDeleteTool) Execute(args map[string]any) Result {
	path := getStr(args, "path")
	recursive, _ := args["recursive"].(bool)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if !inCwd(path) {
		return R("error: cannot delete outside current directory")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return R("error: not found: " + path)
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return R("error: " + err.Error())
	}
	if fi.IsDir() {
		if !recursive {
			return R("error: is a directory — set recursive=true to delete")
		}
		os.RemoveAll(absPath)
		return R("deleted directory: " + path)
	}
	os.Remove(absPath)
	return R("deleted: " + path)
}

// --- FileListTool ---

type FileListTool struct{}

func (t *FileListTool) Name() string { return "file_list" }

func (t *FileListTool) Execute(args map[string]any) Result {
	path := getStr(args, "path")
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if !inCwd(path) {
		return R("error: path is outside allowed scope")
	}

	d, err := os.Open(abs)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer d.Close()

	entries, err := d.Readdir(-1)
	if err != nil {
		return R("error: " + err.Error())
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	ws, _ := filepath.Abs(".")
	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		if e.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(filepath.Join(abs, name))
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(ws, resolved)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
		}
		if e.IsDir() {
			dirs = append(dirs, "["+name+"]")
		} else {
			size := e.Size()
			if kb := float64(size) / 1024; kb >= 1 {
				files = append(files, fmt.Sprintf("%s (%.1fKB)", name, kb))
			} else {
				files = append(files, fmt.Sprintf("%s (%dB)", name, size))
			}
		}
	}
	var b strings.Builder
	if len(dirs) > 0 {
		b.WriteString("dirs: " + strings.Join(dirs, "  "))
	}
	if len(files) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("files: " + strings.Join(files, "  "))
	}
	if b.Len() == 0 {
		return R("(empty)")
	}
	return R(b.String())
}

func execLookPathFn(file string) (string, error) {
	return exec.LookPath(file)
}

func execCommandFn(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
