// Package erasureledger stores completed school erasures outside DB/upload
// volumes. The operator keeps an independent current copy and trusted head.
package erasureledger

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const header = "moto-erasure-ledger-v1\n"

var (
	errLedger     = errors.New("erasure: external ledger unavailable, damaged or unwritable; no overall completion; recover trusted list and retry same order")
	reference     = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,39}$`)
	digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type entry struct {
	SchoolID      int64  `json:"school_id"`
	Reference     string `json:"order_reference"`
	CompletedAt   string `json:"completed_at"`
	ReceiptSHA256 string `json:"receipt_sha256"`
}

type Ledger struct{ path string }

func New(path string) *Ledger { return &Ledger{path: path} }

func Initialize(path string) error {
	if !filepath.IsAbs(path) {
		return errLedger
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errLedger
	}
	defer func() { _ = file.Close() }()
	if _, err = file.WriteString(header); err != nil {
		return errLedger
	}
	if err = file.Sync(); err != nil {
		return errLedger
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return errLedger
	}
	defer func() { _ = dir.Close() }()
	if err = dir.Sync(); err != nil {
		return errLedger
	}
	return nil
}

// Nonblocking locks fail closed instead of hanging an operator session.
// O_NOFOLLOW/O_NONBLOCK reject links and avoid blocking on a substituted FIFO.
func (l *Ledger) open() (*os.File, error) {
	if !filepath.IsAbs(l.path) {
		return nil, errLedger
	}
	file, err := os.OpenFile(l.path, os.O_RDWR|os.O_APPEND|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errLedger
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		_ = file.Close()
		return nil, errLedger
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errLedger
	}
	current, err := os.Lstat(l.path)
	if err != nil || !os.SameFile(info, current) {
		_ = file.Close()
		return nil, errLedger
	}
	return file, nil
}

func checksum(previous, payload string) string {
	sum := sha256.Sum256([]byte(previous + "\n" + payload))
	return hex.EncodeToString(sum[:])
}

func read(file *os.File, visit func(entry) error) (string, error) {
	scanner := bufio.NewScanner(file)
	scanner.Split(terminatedLines)
	scanner.Buffer(make([]byte, 4096), 4096)
	if !scanner.Scan() || scanner.Text()+"\n" != header {
		return "", errLedger
	}
	sum := sha256.Sum256([]byte(header))
	previous := hex.EncodeToString(sum[:])
	schools := map[int64]string{}
	for scanner.Scan() {
		digest, payload, ok := strings.Cut(scanner.Text(), " ")
		if !ok || digest != checksum(previous, payload) || !json.Valid([]byte(payload)) {
			return "", errLedger
		}
		var item entry
		decoder := json.NewDecoder(strings.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&item); err != nil {
			return "", errLedger
		}
		when, err := time.Parse(time.RFC3339Nano, item.CompletedAt)
		if err != nil || when.UTC().Format(time.RFC3339Nano) != item.CompletedAt || item.SchoolID <= 0 || !reference.MatchString(item.Reference) || !digestPattern.MatchString(item.ReceiptSHA256) {
			return "", errLedger
		}
		if order, ok := schools[item.SchoolID]; ok && order != item.Reference {
			return "", errLedger
		}
		schools[item.SchoolID] = item.Reference
		if err := visit(item); err != nil {
			return "", err
		}
		previous = digest
	}
	if scanner.Err() != nil {
		return "", errLedger
	}
	return previous, nil
}

// Unlike bufio.ScanLines, an incomplete final write is an error, not a record.
func terminatedLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) != 0 {
		return 0, nil, errLedger
	}
	return 0, nil, nil
}

func (l *Ledger) Check() error {
	file, err := l.open()
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = read(file, func(entry) error { return nil })
	return err
}

func (l *Ledger) Record(raw json.RawMessage) error {
	var proof struct {
		SchoolID    int64      `json:"school_id"`
		Reference   string     `json:"order_reference"`
		Status      string     `json:"status"`
		CompletedAt *time.Time `json:"completed_at"`
	}
	if err := json.Unmarshal(raw, &proof); err != nil || proof.Status != "completed" || proof.CompletedAt == nil || proof.SchoolID <= 0 || !reference.MatchString(proof.Reference) {
		return errLedger
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return errLedger
	}
	sum := sha256.Sum256(compact.Bytes())
	item := entry{proof.SchoolID, proof.Reference, proof.CompletedAt.UTC().Format(time.RFC3339Nano), hex.EncodeToString(sum[:])}
	file, err := l.open()
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	found := false
	previous, err := read(file, func(old entry) error {
		if old.SchoolID == item.SchoolID && old.Reference != item.Reference {
			return errLedger
		}
		if old == item {
			found = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		payload, err := json.Marshal(item)
		if err != nil {
			return errLedger
		}
		if _, err = fmt.Fprintf(file, "%s %s\n", checksum(previous, string(payload)), payload); err != nil {
			return errLedger
		}
	}
	// Also sync idempotent retries: an earlier sync failure may have left a
	// complete but not yet durable line. Never acknowledge that without fsync.
	if err = file.Sync(); err != nil {
		return errLedger
	}
	return nil
}
