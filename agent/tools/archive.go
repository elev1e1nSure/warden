package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ArchiveTool struct{}

func (t *ArchiveTool) Name() string { return "archive" }

func (t *ArchiveTool) Execute(args map[string]any) Result {
	action := strings.ToLower(getStr(args, "action"))
	path := getStr(args, "path")
	if path == "" {
		return R("error: path is required")
	}
	if action != "list" && action != "extract" && action != "create" {
		return R("error: action must be list, extract, or create")
	}

	format := detectFormat(path)
	if format == "" {
		return R("error: cannot detect archive format from extension (supported: .zip, .tar, .tgz, .tar.gz, .tbz2, .tar.bz2)")
	}

	switch action {
	case "list":
		return t.listArchive(path, format)
	case "extract":
		dest := getStr(args, "dest")
		return t.extractArchive(path, format, dest)
	case "create":
		sources, _ := args["sources"].([]any)
		return t.createArchive(path, format, sources)
	}
	return R("error: unknown action")
}

func (t *ArchiveTool) listArchive(path, format string) Result {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return R("error: not found: " + path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if fi.IsDir() {
		return R("error: not a file: " + path)
	}

	if format == "zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer r.Close()
		var b strings.Builder
		for i, f := range r.File {
			if i >= 200 {
				b.WriteString(fmt.Sprintf("\n... and %d more", len(r.File)-200))
				break
			}
			b.WriteString(fmt.Sprintf("%10d  %s  %s\n", f.UncompressedSize64, f.Modified.Format("2006-01-02 15:04:05"), f.Name))
		}
		if b.Len() == 0 {
			return R("(empty)")
		}
		return R(strings.TrimRight(b.String(), "\n"))
	}

	return t.listTar(path, format)
}

func (t *ArchiveTool) listTar(path, format string) Result {
	f, err := os.Open(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer f.Close()

	var r io.Reader = f
	if format == "tar.gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer gz.Close()
		r = gz
	} else if format == "tar.bz2" {
		r = bzip2.NewReader(f)
	}

	tr := tar.NewReader(r)
	var b strings.Builder
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return R("error: " + err.Error())
		}
		if count >= 200 {
			count++
			continue
		}
		kind := "f"
		if hdr.Typeflag == tar.TypeDir {
			kind = "d"
		}
		b.WriteString(fmt.Sprintf("%10d  %s  %s  %s\n", hdr.Size, time.Unix(hdr.ModTime.Unix(), 0).Format("2006-01-02 15:04:05"), kind, hdr.Name))
		count++
	}
	if count > 200 {
		b.WriteString(fmt.Sprintf("... and %d more", count-200))
	}
	if b.Len() == 0 {
		return R("(empty)")
	}
	return R(strings.TrimRight(b.String(), "\n"))
}

func (t *ArchiveTool) extractArchive(path, format, dest string) Result {
	if dest == "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return R("error: " + err.Error())
		}
		dest = filepath.Dir(abs)
	}
	if !inCwd(dest) {
		return R("error: dest is outside current directory")
	}

	if format == "zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer r.Close()

		// zip slip guard
		destAbs, _ := filepath.Abs(dest)
		for _, f := range r.File {
			target := filepath.Join(destAbs, f.Name)
			if !strings.HasPrefix(filepath.Clean(target), destAbs+string(filepath.Separator)) && target != destAbs {
				return R("error: zip slip detected in member: " + f.Name)
			}
		}
		for _, f := range r.File {
			target := filepath.Join(destAbs, f.Name)
			if f.FileInfo().IsDir() {
				os.MkdirAll(target, 0755)
				continue
			}
			os.MkdirAll(filepath.Dir(target), 0755)
			src, err := f.Open()
			if err != nil {
				return R("error: " + err.Error())
			}
			dst, err := os.Create(target)
			if err != nil {
				src.Close()
				return R("error: " + err.Error())
			}
			io.Copy(dst, src)
			src.Close()
			dst.Close()
		}
		return R("extracted: " + path + " → " + dest)
	}

	return t.extractTar(path, format, dest)
}

func (t *ArchiveTool) extractTar(path, format, dest string) Result {
	f, err := os.Open(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer f.Close()

	var r io.Reader = f
	if format == "tar.gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer gz.Close()
		r = gz
	} else if format == "tar.bz2" {
		r = bzip2.NewReader(f)
	}

	tr := tar.NewReader(r)
	destAbs, _ := filepath.Abs(dest)

	// tar slip guard: read all headers first
	var headers []*tar.Header
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return R("error: " + err.Error())
		}
		target := filepath.Join(destAbs, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), destAbs+string(filepath.Separator)) && target != destAbs {
			return R("error: tar slip detected in member: " + hdr.Name)
		}
		headers = append(headers, hdr)
	}

	// Re-open for extraction
	f.Seek(0, 0)
	if format == "tar.bz2" {
		// Need to re-open for compressed formats
		f.Close()
		f, err = os.Open(path)
		if err != nil {
			return R("error: " + err.Error())
		}
		defer f.Close()
		r = f
		if format == "tar.gz" {
			gz, _ := gzip.NewReader(f)
			defer gz.Close()
			r = gz
		} else {
			r = bzip2.NewReader(f)
		}
		tr = tar.NewReader(r)
	} else {
		f.Seek(0, 0)
		tr = tar.NewReader(f)
	}

	for _, hdr := range headers {
		target := filepath.Join(destAbs, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			dst, err := os.Create(target)
			if err != nil {
				return R("error: " + err.Error())
			}
			if _, err := io.Copy(dst, tr); err != nil {
				dst.Close()
				return R("error: " + err.Error())
			}
			dst.Close()
		}
	}
	return R("extracted: " + path + " → " + dest)
}

func (t *ArchiveTool) createArchive(path, format string, sources []any) Result {
	if len(sources) == 0 {
		return R("error: sources is required for create")
	}

	var srcPaths []string
	var missing []string
	var outside []string
	for _, s := range sources {
		p := fmt.Sprint(s)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			missing = append(missing, p)
		} else if !inCwd(p) {
			outside = append(outside, p)
		} else {
			srcPaths = append(srcPaths, p)
		}
	}
	if len(missing) > 0 {
		return R("error: source(s) not found: " + strings.Join(missing, ", "))
	}
	if len(outside) > 0 {
		return R("error: source(s) outside current directory: " + strings.Join(outside, ", "))
	}
	if !inCwd(path) {
		return R("error: archive path is outside current directory")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	if d := filepath.Dir(absPath); d != "" {
		os.MkdirAll(d, 0755)
	}

	if format == "zip" {
		return t.createZip(absPath, srcPaths)
	}
	return t.createTar(absPath, format, srcPaths)
}

func (t *ArchiveTool) createZip(path string, sources []string) Result {
	zf, err := os.Create(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	count := 0
	for _, src := range sources {
		fi, err := os.Stat(src)
		if err != nil {
			return R("error: " + err.Error())
		}
		if fi.IsDir() {
			filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(filepath.Dir(src), p)
				rel = strings.ReplaceAll(rel, "\\", "/")
				w, err := zw.Create(rel)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				w.Write(data)
				count++
				return nil
			})
		} else {
			w, err := zw.Create(filepath.Base(src))
			if err != nil {
				return R("error: " + err.Error())
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return R("error: " + err.Error())
			}
			w.Write(data)
			count++
		}
	}
	return R(fmt.Sprintf("created: %s (%d entries)", path, count))
}

func (t *ArchiveTool) createTar(path, format string, sources []string) Result {
	f, err := os.Create(path)
	if err != nil {
		return R("error: " + err.Error())
	}
	defer f.Close()

	var w io.WriteCloser = f
	if format == "tar.gz" {
		gw := gzip.NewWriter(f)
		defer gw.Close()
		w = gw
	} else if format == "tar.bz2" {
		return R("error: bzip2 write not supported (use gzip instead)")
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	count := 0
	for _, src := range sources {
		fi, err := os.Stat(src)
		if err != nil {
			return R("error: " + err.Error())
		}
		if fi.IsDir() {
			filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(filepath.Dir(src), p)
				rel = strings.ReplaceAll(rel, "\\", "/")
				if info.IsDir() {
					hdr, _ := tar.FileInfoHeader(info, "")
					hdr.Name = rel + "/"
					tw.WriteHeader(hdr)
					return nil
				}
				hdr, _ := tar.FileInfoHeader(info, "")
				hdr.Name = rel
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				tw.Write(data)
				count++
				return nil
			})
		} else {
			hdr, _ := tar.FileInfoHeader(fi, "")
			hdr.Name = filepath.Base(src)
			if err := tw.WriteHeader(hdr); err != nil {
				return R("error: " + err.Error())
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return R("error: " + err.Error())
			}
			tw.Write(data)
			count++
		}
	}
	return R(fmt.Sprintf("created: %s (%d entries)", path, count))
}

var archiveExts = map[string]string{
	".zip":    "zip",
	".tar":    "tar",
	".tgz":    "tar.gz",
	".tar.gz": "tar.gz",
	".tbz2":   "tar.bz2",
	".tar.bz2": "tar.bz2",
}

func detectFormat(path string) string {
	low := strings.ToLower(path)
	for ext, fmt := range archiveExts {
		if strings.HasSuffix(low, ext) {
			return fmt
		}
	}
	return ""
}
