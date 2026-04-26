package pdf

import "time"

type SourceDocument struct {
	Path      string
	Name      string
	SizeBytes int64
	SHA256    string
	LoadedAt  time.Time
	RawBytes  []byte
}
