package cache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/mmap"

	"github.com/binsarjr/regionchecker/internal/rir"
)

// Snapshot is a memory-mapped, parsed RCHK snapshot ready for lookup.
type Snapshot struct {
	DB       *rir.DB
	Path     string
	Size     int64
	reader   *mmap.ReaderAt
	raw      []byte
}

// LoadSnapshot mmaps the file at path, parses the RCHK header and rows via
// rir.LoadSnapshot, and returns a typed view. Caller MUST Close when done.
func LoadSnapshot(path string) (*Snapshot, error) {
	r, err := mmap.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cache: mmap open: %w", err)
	}
	size := int64(r.Len())
	buf := make([]byte, size)
	if _, err := r.ReadAt(buf, 0); err != nil && err != io.EOF {
		_ = r.Close()
		return nil, fmt.Errorf("cache: mmap read: %w", err)
	}
	db, err := rir.LoadSnapshot(bytes.NewReader(buf), [32]byte{})
	if err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("cache: parse snapshot: %w", err)
	}
	return &Snapshot{DB: db, Path: path, Size: size, reader: r, raw: buf}, nil
}

// Close releases the mmap.
func (s *Snapshot) Close() error {
	if s == nil || s.reader == nil {
		return nil
	}
	err := s.reader.Close()
	s.reader = nil
	s.raw = nil
	return err
}

// VerifySHA256 reports whether sha256(raw) == expected.
func VerifySHA256(raw []byte, expected [32]byte) bool {
	got := sha256.Sum256(raw)
	return got == expected
}

// Stat reports the on-disk size for path.
func snapshotSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
