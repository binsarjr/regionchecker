package rir

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	snapshotSchemaVersion = 1
)

var snapshotMagic = [4]byte{'R', 'C', 'H', 'K'}

// Snapshot serializes db into a custom binary format for fast reload.
//
// Layout:
//
//	magic[4]="RCHK" | version u32 | rawSHA[32] | reserved[16]
//	v4Count u32 | v4 rows... (24 bytes each)
//	v6Count u32 | v6 rows... (44 bytes each)
//	asnCount u32 | asn rows... (16 bytes each)
//
// rawSHA identifies the source delegated file. If it mismatches on load, the
// caller should rebuild from raw.
func Snapshot(db *DB, rawSHA [32]byte, w io.Writer) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.Write(snapshotMagic[:]); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(snapshotSchemaVersion)); err != nil {
		return err
	}
	if _, err := bw.Write(rawSHA[:]); err != nil {
		return err
	}
	var reserved [16]byte
	if _, err := bw.Write(reserved[:]); err != nil {
		return err
	}
	if err := writeV4(bw, db.V4); err != nil {
		return err
	}
	if err := writeV6(bw, db.V6); err != nil {
		return err
	}
	if err := writeASN(bw, db.ASN); err != nil {
		return err
	}
	return bw.Flush()
}

// LoadSnapshot reads a snapshot produced by Snapshot. If expectedSHA is
// non-zero and mismatches the recorded rawSHA, ErrSnapshotStale is returned.
func LoadSnapshot(r io.Reader, expectedSHA [32]byte) (*DB, error) {
	br := bufio.NewReader(r)
	var magic [4]byte
	if _, err := io.ReadFull(br, magic[:]); err != nil {
		return nil, fmt.Errorf("snapshot: read magic: %w", err)
	}
	if magic != snapshotMagic {
		return nil, fmt.Errorf("snapshot: bad magic %v", magic)
	}
	var ver uint32
	if err := binary.Read(br, binary.LittleEndian, &ver); err != nil {
		return nil, err
	}
	if ver != snapshotSchemaVersion {
		return nil, fmt.Errorf("snapshot: version %d unsupported", ver)
	}
	var gotSHA [32]byte
	if _, err := io.ReadFull(br, gotSHA[:]); err != nil {
		return nil, err
	}
	var zero [32]byte
	if expectedSHA != zero && gotSHA != expectedSHA {
		return nil, ErrSnapshotStale
	}
	var reserved [16]byte
	if _, err := io.ReadFull(br, reserved[:]); err != nil {
		return nil, err
	}
	db := &DB{}
	var err error
	if db.V4, err = readV4(br); err != nil {
		return nil, err
	}
	if db.V6, err = readV6(br); err != nil {
		return nil, err
	}
	if db.ASN, err = readASN(br); err != nil {
		return nil, err
	}
	return db, nil
}

// ErrSnapshotStale signals the source file has changed since the snapshot was
// built; caller should rebuild.
var ErrSnapshotStale = errors.New("rir: snapshot stale")

// SHA256 hashes data for snapshot provenance tracking.
func SHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// SHA256Reader consumes r and returns its sha256.
func SHA256Reader(r io.Reader) ([32]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return [32]byte{}, err
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func writeV4(w io.Writer, rows []V4Range) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(rows))); err != nil {
		return err
	}
	buf := make([]byte, 24)
	for _, r := range rows {
		binary.LittleEndian.PutUint32(buf[0:4], r.Start)
		binary.LittleEndian.PutUint32(buf[4:8], r.End)
		buf[8] = r.CC[0]
		buf[9] = r.CC[1]
		buf[10] = byte(r.Registry)
		buf[11] = byte(r.Status)
		binary.LittleEndian.PutUint32(buf[12:16], r.Date)
		// 8 bytes reserved for future fields
		for i := 16; i < 24; i++ {
			buf[i] = 0
		}
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func readV4(r io.Reader) ([]V4Range, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	rows := make([]V4Range, n)
	buf := make([]byte, 24)
	for i := range rows {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		rows[i] = V4Range{
			Start:    binary.LittleEndian.Uint32(buf[0:4]),
			End:      binary.LittleEndian.Uint32(buf[4:8]),
			CC:       [2]byte{buf[8], buf[9]},
			Registry: RegistryID(buf[10]),
			Status:   Status(buf[11]),
			Date:     binary.LittleEndian.Uint32(buf[12:16]),
		}
	}
	return rows, nil
}

func writeV6(w io.Writer, rows []V6Range) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(rows))); err != nil {
		return err
	}
	buf := make([]byte, 44)
	for _, r := range rows {
		binary.LittleEndian.PutUint64(buf[0:8], r.StartHi)
		binary.LittleEndian.PutUint64(buf[8:16], r.StartLo)
		binary.LittleEndian.PutUint64(buf[16:24], r.EndHi)
		binary.LittleEndian.PutUint64(buf[24:32], r.EndLo)
		buf[32] = r.CC[0]
		buf[33] = r.CC[1]
		buf[34] = byte(r.Registry)
		buf[35] = byte(r.Status)
		binary.LittleEndian.PutUint32(buf[36:40], r.Date)
		for i := 40; i < 44; i++ {
			buf[i] = 0
		}
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func readV6(r io.Reader) ([]V6Range, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	rows := make([]V6Range, n)
	buf := make([]byte, 44)
	for i := range rows {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		rows[i] = V6Range{
			StartHi:  binary.LittleEndian.Uint64(buf[0:8]),
			StartLo:  binary.LittleEndian.Uint64(buf[8:16]),
			EndHi:    binary.LittleEndian.Uint64(buf[16:24]),
			EndLo:    binary.LittleEndian.Uint64(buf[24:32]),
			CC:       [2]byte{buf[32], buf[33]},
			Registry: RegistryID(buf[34]),
			Status:   Status(buf[35]),
			Date:     binary.LittleEndian.Uint32(buf[36:40]),
		}
	}
	return rows, nil
}

func writeASN(w io.Writer, rows []ASNRange) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(rows))); err != nil {
		return err
	}
	buf := make([]byte, 16)
	for _, r := range rows {
		binary.LittleEndian.PutUint32(buf[0:4], r.Start)
		binary.LittleEndian.PutUint32(buf[4:8], r.End)
		buf[8] = r.CC[0]
		buf[9] = r.CC[1]
		buf[10] = byte(r.Registry)
		buf[11] = byte(r.Status)
		binary.LittleEndian.PutUint32(buf[12:16], r.Date)
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func readASN(r io.Reader) ([]ASNRange, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	rows := make([]ASNRange, n)
	buf := make([]byte, 16)
	for i := range rows {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		rows[i] = ASNRange{
			Start:    binary.LittleEndian.Uint32(buf[0:4]),
			End:      binary.LittleEndian.Uint32(buf[4:8]),
			CC:       [2]byte{buf[8], buf[9]},
			Registry: RegistryID(buf[10]),
			Status:   Status(buf[11]),
			Date:     binary.LittleEndian.Uint32(buf[12:16]),
		}
	}
	return rows, nil
}

// ensure import used
var _ = bytes.NewReader
