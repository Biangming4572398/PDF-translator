package pdf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotPDF   = errors.New("selected file is not a PDF")
	ErrTooLarge = errors.New("selected PDF exceeds the configured size limit")
)

type Loader struct {
	MaxBytes int64
}

func NewLoader(maxFileSizeMB int) *Loader {
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = 20
	}

	return &Loader{
		MaxBytes: int64(maxFileSizeMB) * 1024 * 1024,
	}
}

func (l *Loader) Load(path string) (SourceDocument, error) {
	if strings.ToLower(filepath.Ext(path)) != ".pdf" {
		return SourceDocument{}, ErrNotPDF
	}

	info, err := os.Stat(path)
	if err != nil {
		return SourceDocument{}, err
	}

	if info.Size() > l.MaxBytes {
		return SourceDocument{}, fmt.Errorf("%w: %.2f MB limit", ErrTooLarge, float64(l.MaxBytes)/(1024*1024))
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return SourceDocument{}, err
	}

	sum := sha256.Sum256(raw)
	return SourceDocument{
		Path:      path,
		Name:      info.Name(),
		SizeBytes: info.Size(),
		SHA256:    hex.EncodeToString(sum[:]),
		LoadedAt:  time.Now(),
		RawBytes:  raw,
	}, nil
}
