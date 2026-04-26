package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type PDFListItem struct {
	Name     string
	Path     string
	Size     int64
	Modified time.Time
}

func defaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "Downloads")
}

func scanPDFs(dir string) ([]PDFListItem, error) {
	if dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	items := make([]PDFListItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".pdf" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		items = append(items, PDFListItem{
			Name:     entry.Name(),
			Path:     filepath.Join(dir, entry.Name()),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Modified.After(items[j].Modified)
	})

	return items, nil
}

func formatBytes(size int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.2f GB", float64(size)/gb)
	case size >= mb:
		return fmt.Sprintf("%.2f MB", float64(size)/mb)
	case size >= kb:
		return fmt.Sprintf("%.2f KB", float64(size)/kb)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}

func joinMessages(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

func openPathInSystem(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
