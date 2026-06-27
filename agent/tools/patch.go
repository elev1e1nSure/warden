package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	patchHeaderRe = regexp.MustCompile(`^--- (?:\S+)`)
	patchDelimRe  = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)`)

	ocBeginRe  = regexp.MustCompile(`^\*\*\* Begin Patch\s*$`)
	ocEndRe    = regexp.MustCompile(`^\*\*\* End Patch\s*$`)
	ocUpdateRe = regexp.MustCompile(`^\*\*\* Update File:\s*(.+?)\s*$`)
	ocAddRe    = regexp.MustCompile(`^\*\*\* Add File:\s*(.+?)\s*$`)
	ocDeleteRe = regexp.MustCompile(`^\*\*\* Delete File:\s*(.+?)\s*$`)
	ocMoveRe   = regexp.MustCompile(`^\*\*\* Move to:\s*(.+?)\s*$`)
	ocEOFRe    = regexp.MustCompile(`^\*\*\* End of File\s*$`)
)

type patchFile struct {
	path     string
	oldPath  string
	newPath  string
	isAdd    bool
	isDelete bool
	isRename bool
	hunks    []patchHunk
}

type patchHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

type ocFile struct {
	kind    string // "update", "add", "delete"
	path    string
	moveTo  string
	hunks   []ocHunk
}

type ocHunk struct {
	oldLines []string
	newLines []string
	atEOF    bool
}

// --- ApplyPatchTool ---

type ApplyPatchTool struct{}

func (t *ApplyPatchTool) Name() string { return "apply_patch" }

func (t *ApplyPatchTool) Execute(args map[string]any) Result {
	patch := getStr(args, "patch_text")
	if patch == "" {
		return R("error: patch_text is required")
	}
	patch = strings.ReplaceAll(patch, "\r\n", "\n")
	patch = strings.ReplaceAll(patch, "\r", "\n")

	if strings.Contains(patch, "*** Begin Patch") {
		return t.executeOpencode(patch)
	}

	files := t.parsePatch(patch)
	if len(files) == 0 {
		return R("error: no valid hunks found in patch")
	}

	added := 0
	removed := 0
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	var results []string
	for _, f := range files {
		results = append(results, t.applyFile(f))
	}

	out := strings.Join(results, "\n")
	stats := ""
	if added > 0 || removed > 0 {
		stats = fmt.Sprintf("+%d -%d", added, removed)
	}
	label := out
	if stats != "" {
		label = out + "  " + stats
	}
	if added > 0 || removed > 0 {
		return RD(label, patch)
	}
	return R(label)
}

func (t *ApplyPatchTool) parsePatch(patch string) []patchFile {
	var files []patchFile
	lines := strings.Split(patch, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := regexp.MustCompile(`^--- (?:"([^"]+)"|(\S+))`).FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}
		oldPath := m[1]
		if oldPath == "" {
			oldPath = m[2]
		}
		i++
		if i >= len(lines) {
			break
		}
		m2 := regexp.MustCompile(`^\+\+\+ (?:"([^"]+)"|(\S+))`).FindStringSubmatch(lines[i])
		if m2 == nil {
			continue
		}
		newPath := m2[1]
		if newPath == "" {
			newPath = m2[2]
		}
		i++

		isAdd := oldPath == "/dev/null"
		isDelete := newPath == "/dev/null"
		isRename := !isAdd && !isDelete && oldPath != newPath

		target := newPath
		if newPath == "/dev/null" {
			target = oldPath
		}

		var hunks []patchHunk
		for i < len(lines) {
			h := patchDelimRe.FindStringSubmatch(lines[i])
			if h == nil {
				if patchHeaderRe.MatchString(lines[i]) {
					break
				}
				i++
				continue
			}
			oldStart := atoi(h[1])
			oldCount := 1
			if h[2] != "" {
				oldCount = atoi(h[2])
			}
			newStart := atoi(h[3])
			newCount := 1
			if h[4] != "" {
				newCount = atoi(h[4])
			}
			i++

			var hunkLines []string
			for i < len(lines) {
				ln := lines[i]
				if patchHeaderRe.MatchString(ln) || strings.HasPrefix(ln, "@@ ") {
					break
				}
				hunkLines = append(hunkLines, ln)
				i++
			}

			hunks = append(hunks, patchHunk{
				oldStart: oldStart, oldCount: oldCount,
				newStart: newStart, newCount: newCount,
				lines: hunkLines,
			})
		}

		path := strings.TrimLeft(target, "/")
		path = regexp.MustCompile(`^[ab]/`).ReplaceAllString(path, "")
		if matched, _ := regexp.MatchString(`^[a-zA-Z]:/`, path); !matched {
			path = strings.TrimLeft(path, "/")
		}

		files = append(files, patchFile{
			path: path, oldPath: oldPath, newPath: newPath,
			isAdd: isAdd, isDelete: isDelete, isRename: isRename,
			hunks: hunks,
		})
	}
	return files
}

func (t *ApplyPatchTool) applyFile(f patchFile) string {
	path, err := filepath.Abs(f.path)
	if err != nil {
		return fmt.Sprintf("blocked: %s — %v", f.path, err)
	}
	if !inCwd(f.path) {
		return fmt.Sprintf("blocked: %s — outside workspace", f.path)
	}

	if f.isDelete {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Sprintf("delete: %s — not found (skipped)", f.path)
		}
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			return fmt.Sprintf("delete: %s — is a directory (skipped)", f.path)
		}
		os.Remove(path)
		return fmt.Sprintf("deleted: %s", f.path)
	}

	var content string
	if f.isRename {
		oldPath := strings.TrimLeft(f.oldPath, "/")
		oldPath = filepath.Clean(oldPath)
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			return fmt.Sprintf("rename: %s → %s — source not found (skipped)", f.oldPath, f.path)
		}
		data, err := os.ReadFile(oldPath)
		if err != nil {
			return fmt.Sprintf("rename: %s — read error: %v", f.oldPath, err)
		}
		content = string(data)
		os.Remove(oldPath)
	} else if f.isAdd {
		content = ""
	} else {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Sprintf("patch: %s — not found (skipped)", f.path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Sprintf("patch: %s — read error: %v", f.path, err)
		}
		content = string(data)
	}

	delta := 0
	for _, hunk := range f.hunks {
		adjusted := hunk
		adjusted.oldStart = hunk.oldStart + delta
		result := applyHunk(content, adjusted)
		if result == nil {
			return fmt.Sprintf("patch: %s — hunk @@ -%d,%d +%d,%d @@ failed to match",
				f.path, hunk.oldStart, hunk.oldCount, hunk.newStart, hunk.newCount)
		}
		content = *result
		oldLines := 0
		newLines := 0
		for _, hl := range hunk.lines {
			if len(hl) == 0 || hl[0] == ' ' || hl[0] == '-' {
				oldLines++
			}
			if len(hl) == 0 || hl[0] == ' ' || hl[0] == '+' {
				newLines++
			}
		}
		delta += newLines - oldLines
	}

	if d := filepath.Dir(path); d != "" {
		os.MkdirAll(d, 0755)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Sprintf("patch: %s — write error: %v", f.path, err)
	}

	switch {
	case f.isAdd:
		return fmt.Sprintf("added: %s", f.path)
	case f.isRename:
		return fmt.Sprintf("renamed: %s → %s", f.oldPath, f.path)
	default:
		return fmt.Sprintf("patched: %s", f.path)
	}
}

func applyHunk(content string, hunk patchHunk) *string {
	lines := strings.Split(content, "\n")
	oldStart := hunk.oldStart - 1
	if oldStart < 0 {
		oldStart = 0
	}

	var oldLines, newLines []string
	for _, ln := range hunk.lines {
		if ln == "" {
			oldLines = append(oldLines, "")
			newLines = append(newLines, "")
		} else if ln[0] == ' ' {
			oldLines = append(oldLines, ln[1:])
			newLines = append(newLines, ln[1:])
		} else if ln[0] == '-' {
			oldLines = append(oldLines, ln[1:])
		} else if ln[0] == '+' {
			newLines = append(newLines, ln[1:])
		}
	}

	if len(oldLines) == 0 {
		insertAt := oldStart
		if insertAt > len(lines) {
			insertAt = len(lines)
		}
		result := make([]string, 0, len(lines)+len(newLines))
		result = append(result, lines[:insertAt]...)
		result = append(result, newLines...)
		result = append(result, lines[insertAt:]...)
		s := strings.Join(result, "\n")
		return &s
	}

	// Search for match
	matchStart := -1
	order := make([]int, len(lines)+1)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		da := order[a] - oldStart
		if da < 0 {
			da = -da
		}
		db := order[b] - oldStart
		if db < 0 {
			db = -db
		}
		return da < db
	})
	for _, i := range order {
		if i+len(oldLines) > len(lines) {
			continue
		}
		match := true
		for j, ol := range oldLines {
			if lines[i+j] != ol {
				match = false
				break
			}
		}
		if match {
			matchStart = i
			break
		}
	}
	if matchStart == -1 {
		return nil
	}

	result := make([]string, 0, len(lines)-len(oldLines)+len(newLines))
	result = append(result, lines[:matchStart]...)
	result = append(result, newLines...)
	result = append(result, lines[matchStart+len(oldLines):]...)
	s := strings.Join(result, "\n")
	return &s
}

// --- opencode / Anthropic format ---

func (t *ApplyPatchTool) executeOpencode(patch string) Result {
	normalized := strings.TrimSpace(patch)
	if normalized == "*** Begin Patch\n*** End Patch" {
		return R("error: empty patch")
	}
	files := t.parseOpencode(patch)
	if len(files) == 0 {
		return R("error: no valid hunks found in patch")
	}

	var results []string
	totalAdded := 0
	totalRemoved := 0
	for _, f := range files {
		r, a, d := t.applyOpencodeFile(f)
		results = append(results, r)
		totalAdded += a
		totalRemoved += d
	}

	out := strings.Join(results, "\n")
	stats := ""
	if totalAdded > 0 || totalRemoved > 0 {
		stats = fmt.Sprintf("+%d -%d", totalAdded, totalRemoved)
	}
	label := out
	if stats != "" {
		label = out + "  " + stats
	}
	if totalAdded > 0 || totalRemoved > 0 {
		return RD(label, patch)
	}
	return R(label)
}

func (t *ApplyPatchTool) parseOpencode(patch string) []ocFile {
	lines := strings.Split(patch, "\n")
	i := 0
	for i < len(lines) && !ocBeginRe.MatchString(lines[i]) {
		i++
	}
	i++

	var files []ocFile
	var current *ocFile
	atEOF := false
	var oldLines, newLines []string

	flushHunk := func() {
		if current == nil {
			oldLines, newLines = nil, nil
			atEOF = false
			return
		}
		current.hunks = append(current.hunks, ocHunk{
			oldLines: oldLines,
			newLines: newLines,
			atEOF:    atEOF,
		})
		oldLines, newLines = nil, nil
		atEOF = false
	}

	flushFile := func() {
		flushHunk()
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}

	for i < len(lines) {
		line := lines[i]
		if ocEndRe.MatchString(line) {
			flushFile()
			break
		}
		if m := ocUpdateRe.FindStringSubmatch(line); m != nil {
			flushFile()
			current = &ocFile{kind: "update", path: normalizePath(m[1])}
			i++
			continue
		}
		if m := ocAddRe.FindStringSubmatch(line); m != nil {
			flushFile()
			current = &ocFile{kind: "add", path: normalizePath(m[1])}
			i++
			continue
		}
		if m := ocDeleteRe.FindStringSubmatch(line); m != nil {
			flushFile()
			current = &ocFile{kind: "delete", path: normalizePath(m[1])}
			i++
			continue
		}
		if m := ocMoveRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				current.moveTo = normalizePath(m[1])
			}
			i++
			continue
		}
		if ocEOFRe.MatchString(line) {
			atEOF = true
			i++
			continue
		}
		if current == nil {
			i++
			continue
		}
		if line == "" {
			oldLines = append(oldLines, "")
			newLines = append(newLines, "")
			i++
			continue
		}
		prefix := line[0]
		switch prefix {
		case ' ':
			oldLines = append(oldLines, line[1:])
			newLines = append(newLines, line[1:])
		case '-':
			oldLines = append(oldLines, line[1:])
		case '+':
			newLines = append(newLines, line[1:])
		}
		i++
	}
	flushFile()
	return files
}

func (t *ApplyPatchTool) applyOpencodeFile(f ocFile) (string, int, int) {
	path, err := filepath.Abs(f.path)
	if err != nil {
		return fmt.Sprintf("blocked: %s — %v", f.path, err), 0, 0
	}
	if !inCwd(f.path) {
		return fmt.Sprintf("blocked: %s — outside workspace", f.path), 0, 0
	}

	if f.kind == "delete" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Sprintf("delete: %s — not found (skipped)", f.path), 0, 0
		}
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			return fmt.Sprintf("delete: %s — is a directory (skipped)", f.path), 0, 0
		}
		os.Remove(path)
		return fmt.Sprintf("deleted: %s", f.path), 0, 0
	}

	if f.kind == "add" {
		var b strings.Builder
		added := 0
		for _, h := range f.hunks {
			for _, nl := range h.newLines {
				b.WriteString(nl)
				b.WriteString("\n")
				added++
			}
			if !h.atEOF {
				b.WriteString("\n")
			}
		}
		content := b.String()
		// Remove trailing double newline if we added one too many
		if strings.HasSuffix(content, "\n\n") {
			content = strings.TrimSuffix(content, "\n")
		}
		if d := filepath.Dir(path); d != "" {
			os.MkdirAll(d, 0755)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Sprintf("add: %s — write error: %v", f.path, err), 0, 0
		}
		return fmt.Sprintf("added: %s", f.path), added, 0
	}

	// update
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Sprintf("patch: %s — not found (skipped)", f.path), 0, 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("patch: %s — read error: %v", f.path, err), 0, 0
	}
	raw := string(data)
	hasTrailing := strings.HasSuffix(raw, "\n")
	if hasTrailing {
		raw = strings.TrimSuffix(raw, "\n")
	}
	lines := strings.Split(raw, "\n")

	added, removed := 0, 0
	for _, h := range f.hunks {
		removed += len(h.oldLines)
		added += len(h.newLines)
		result := applyOpencodeHunk(lines, h)
		if result == nil {
			return fmt.Sprintf("patch: %s — hunk failed to match", f.path), 0, 0
		}
		lines = result
	}

	newContent := strings.Join(lines, "\n")
	if hasTrailing {
		newContent += "\n"
	}

	if d := filepath.Dir(path); d != "" {
		os.MkdirAll(d, 0755)
	}
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Sprintf("patch: %s — write error: %v", f.path, err), 0, 0
	}

	if f.moveTo != "" {
		newPath, err := filepath.Abs(f.moveTo)
		if err != nil {
			return fmt.Sprintf("blocked: move to %s — %v", f.moveTo, err), 0, 0
		}
		if !inCwd(f.moveTo) {
			return fmt.Sprintf("blocked: move to %s — outside workspace", f.moveTo), 0, 0
		}
		if d := filepath.Dir(newPath); d != "" {
			os.MkdirAll(d, 0755)
		}
		if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
			return fmt.Sprintf("rename: %s → %s — write error: %v", f.path, f.moveTo, err), 0, 0
		}
		os.Remove(path)
		return fmt.Sprintf("renamed: %s → %s", f.path, f.moveTo), added, removed
	}

	return fmt.Sprintf("patched: %s", f.path), added, removed
}

func applyOpencodeHunk(lines []string, hunk ocHunk) []string {
	oldLines := hunk.oldLines
	newLines := hunk.newLines
	atEOF := hunk.atEOF

	if len(oldLines) == 0 {
		if atEOF {
			return append(lines, newLines...)
		}
		return append(lines, newLines...)
	}

	matchStart := -1
	for i := 0; i <= len(lines)-len(oldLines); i++ {
		match := true
		for j, ol := range oldLines {
			if lines[i+j] != ol {
				match = false
				break
			}
		}
		if match {
			matchStart = i
			if !atEOF {
				break
			}
		}
	}

	if matchStart == -1 {
		return nil
	}

	if atEOF && matchStart+len(oldLines) != len(lines) {
		for i := len(lines) - len(oldLines); i >= 0; i-- {
			match := true
			for j, ol := range oldLines {
				if lines[i+j] != ol {
					match = false
					break
				}
			}
			if match {
				matchStart = i
				break
			}
		}
		if matchStart+len(oldLines) != len(lines) {
			return nil
		}
	}

	result := make([]string, 0, len(lines)-len(oldLines)+len(newLines))
	result = append(result, lines[:matchStart]...)
	result = append(result, newLines...)
	result = append(result, lines[matchStart+len(oldLines):]...)
	return result
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimLeft(p, "/")
	p = regexp.MustCompile(`^[ab]/`).ReplaceAllString(p, "")
	if matched, _ := regexp.MatchString(`^[a-zA-Z]:[\\/]`, p); matched {
		return p
	}
	return p
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
