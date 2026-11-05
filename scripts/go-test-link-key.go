// go-test-link-key normalizes link inputs by complete archive build IDs.
// cmd/go's link action uses archive content IDs, which can remain unchanged
// after comment-only edits. Full IDs also include each compile action's ID.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var archiveID = regexp.MustCompile(`^[A-Za-z0-9_-]{20}/[A-Za-z0-9_-]{20}$`)

func buildID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// Go archives put their build ID in __.PKGDEF, before the export data.
	// Unknown archive formats disable caching rather than guessing an identity.
	header := make([]byte, 4096)
	n, err := io.ReadFull(f, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	header = header[:n]
	if !bytes.HasPrefix(header, []byte("!<arch>\n")) {
		return "", fmt.Errorf("unknown Go archive format")
	}
	_, rest, ok := bytes.Cut(header, []byte("\nbuild id \""))
	if !ok {
		return "", fmt.Errorf("missing Go archive build ID")
	}
	id, _, ok := bytes.Cut(rest, []byte("\"\n"))
	if !ok || !archiveID.Match(id) {
		return "", fmt.Errorf("unknown Go archive build ID")
	}
	return string(id), nil
}

func key(config, mainArchive string) (string, error) {
	f, err := os.Open(config)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	scanner := bufio.NewScanner(f)
	// modinfo is one quoted line and can exceed Scanner's default limit.
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "packagefile "):
			name, path, ok := strings.Cut(line, "=")
			if !ok {
				return "", fmt.Errorf("invalid packagefile")
			}
			id, err := buildID(path)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(h, "%q %q\n", name, id)
		case strings.HasPrefix(line, "modinfo "):
			fmt.Fprintln(h, line)
		default:
			return "", fmt.Errorf("unknown link input")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	id, err := buildID(mainArchive)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(h, id)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func main() {
	if len(os.Args) != 3 {
		os.Exit(1)
	}
	id, err := key(os.Args[1], os.Args[2])
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(id)
}
