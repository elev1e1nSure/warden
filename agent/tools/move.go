package tools

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileMoveTool struct{}

func (t *FileMoveTool) Name() string { return "file_move" }

func (t *FileMoveTool) Execute(args map[string]any) Result {
	src, _ := args["src"].(string)
	dest, _ := args["dest"].(string)
	if src == "" || dest == "" {
		return R("error: src and dest are required")
	}
	if !inCwd(src) || !inCwd(dest) {
		return R("error: path is outside current directory")
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return R("error: " + err.Error())
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return R("error: " + err.Error())
	}
	if _, err := os.Stat(absSrc); os.IsNotExist(err) {
		return R("error: source not found: " + src)
	}
	if absSrc != absDest && strings.HasPrefix(absDest, absSrc+string(filepath.Separator)) {
		return R("error: cannot move " + src + " into itself")
	}
	if d := filepath.Dir(absDest); d != "" {
		os.MkdirAll(d, 0755)
	}
	if err := os.Rename(absSrc, absDest); err != nil {
		if copyErr := copyMove(absSrc, absDest); copyErr != nil {
			return R("error: " + copyErr.Error())
		}
		os.RemoveAll(absSrc)
	}
	return R("moved: " + src + " → " + dest)
}

type FileCopyTool struct{}

func (t *FileCopyTool) Name() string { return "file_copy" }

func (t *FileCopyTool) Execute(args map[string]any) Result {
	src, _ := args["src"].(string)
	dest, _ := args["dest"].(string)
	if src == "" || dest == "" {
		return R("error: src and dest are required")
	}
	if !inCwd(src) || !inCwd(dest) {
		return R("error: path is outside current directory")
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return R("error: " + err.Error())
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return R("error: " + err.Error())
	}
	fi, err := os.Stat(absSrc)
	if os.IsNotExist(err) {
		return R("error: source not found: " + src)
	}
	if fi.IsDir() {
		return R("error: source is a directory (only file copy is supported): " + src)
	}
	if d := filepath.Dir(absDest); d != "" {
		os.MkdirAll(d, 0755)
	}
	if err := copyFile(absSrc, absDest); err != nil {
		return R("error: " + err.Error())
	}
	return R("copied: " + src + " → " + dest)
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return d.Close()
}

func copyMove(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
