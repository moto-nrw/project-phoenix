// Package datareturn releases a checked return package for retrieval and
// serves it to the holder of a token (Art. 30 Abs. 2 Data Act, ADR 0203).
package datareturn

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

// checkedPackage is a package directory whose COMPLETE marker, manifest and
// every listed file were verified, and whose subject is the one the
// operator named.
type checkedPackage struct {
	dir            string
	kind           string
	subjectID      int64
	manifestSHA256 string
	completeSHA256 string
	formatVersion  int
	artifacts      []organizationtenancy.ExportArtifact
}

// The fixed files of a package beside the listed artifacts.
const (
	manifestFile = "manifest.json"
	completeFile = "COMPLETE"
)

// checkPackage refuses anything but a COMPLETE package of the named school
// or Träger: a person review, a changed or extra file, a link, another
// subject. It is the gate between the export and the retrieval; the
// operator still checks the content as the runbook says.
func checkPackage(dir string, schoolID, organizationID int64, fingerprint func() (io.Writer, func() string)) (checkedPackage, error) {
	checked := checkedPackage{dir: dir}
	if (schoolID > 0) == (organizationID > 0) {
		return checked, errors.New("data return: exactly one school or Träger is required")
	}
	manifestData, err := readRegular(dir, manifestFile)
	if err != nil {
		return checked, err
	}
	complete, err := readRegular(dir, completeFile)
	if err != nil {
		return checked, errors.New("data return: the package has no COMPLETE marker; never release a failed export")
	}
	checked.manifestSHA256 = sum(fingerprint, manifestData)
	checked.completeSHA256 = sum(fingerprint, complete)
	if strings.TrimSpace(string(complete)) != checked.manifestSHA256 {
		return checked, errors.New("data return: COMPLETE does not match manifest.json")
	}
	var manifest organizationtenancy.ExportManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return checked, errors.New("data return: manifest.json is not valid")
	}
	if manifest.ReviewRequired {
		return checked, errors.New("data return: a person review workspace is never released")
	}
	if manifest.FormatVersion != organizationtenancy.ExportFormatVersion {
		return checked, fmt.Errorf("data return: format version %d is not the version of this build", manifest.FormatVersion)
	}
	checked.formatVersion = manifest.FormatVersion
	if err := checkArtifacts(dir, manifest.Artifacts, fingerprint); err != nil {
		return checked, err
	}
	checked.artifacts = manifest.Artifacts
	checked.kind, checked.subjectID, err = packageSubject(dir, manifest)
	if err != nil {
		return checked, err
	}
	if checked.kind == "school" && checked.subjectID != schoolID || checked.kind == "organization" && checked.subjectID != organizationID {
		return checked, errors.New("data return: the package belongs to another school or Träger than the one named")
	}
	return checked, nil
}

func sum(fingerprint func() (io.Writer, func() string), data []byte) string {
	w, digest := fingerprint()
	_, _ = w.Write(data)
	return digest()
}

// readRegular reads a regular file of the package, never through a link.
func readRegular(dir, name string) ([]byte, error) {
	full := filepath.Join(dir, filepath.FromSlash(name))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("data return: %s is missing or not a regular file", name)
	}
	return os.ReadFile(full)
}

// checkArtifacts verifies every file the manifest lists, then that the
// directory holds nothing else.
func checkArtifacts(dir string, artifacts []organizationtenancy.ExportArtifact, fingerprint func() (io.Writer, func() string)) error {
	listed := map[string]bool{manifestFile: true, completeFile: true}
	for _, artifact := range artifacts {
		if !validArtifactPath(artifact.Path) || listed[artifact.Path] {
			return errors.New("data return: the manifest lists an invalid or repeated path")
		}
		listed[artifact.Path] = true
		if !artifactMatches(dir, artifact, fingerprint) {
			return errors.New("data return: a file differs from its checksum in the manifest")
		}
	}
	return filepath.WalkDir(dir, func(full string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(dir, full)
		if !entry.Type().IsRegular() || relErr != nil || !listed[filepath.ToSlash(rel)] {
			return errors.New("data return: the package holds a link, a special file or a file the manifest does not list")
		}
		return nil
	})
}

func validArtifactPath(name string) bool {
	return name != "" && !path.IsAbs(name) && path.Clean(name) == name && !strings.HasPrefix(name, "../")
}

// artifactMatches reads a listed file, never through a link, and compares
// its size and SHA-256 with the manifest.
func artifactMatches(dir string, artifact organizationtenancy.ExportArtifact, fingerprint func() (io.Writer, func() string)) bool {
	full := filepath.Join(dir, filepath.FromSlash(artifact.Path))
	if info, err := os.Lstat(full); err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.Open(full)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	w, digest := fingerprint()
	n, err := io.Copy(w, file)
	return err == nil && n == artifact.Bytes && digest() == artifact.SHA256
}

// packageSubject reads the school or Träger the package itself names: the
// one row of platform.schools in a school package, the one row of
// platform.organizations in a Träger package.
func packageSubject(dir string, manifest organizationtenancy.ExportManifest) (string, int64, error) {
	kind, table := "school", "platform.schools.jsonl"
	if manifest.Package == "organization" {
		kind, table = "organization", "platform.organizations.jsonl"
	}
	data, err := readRegular(dir, table)
	if err != nil {
		return kind, 0, err
	}
	var ids []int64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		var row struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return kind, 0, errors.New("data return: the subject table is not valid")
		}
		ids = append(ids, row.ID)
	}
	if scanner.Err() != nil || len(ids) != 1 || ids[0] <= 0 {
		return kind, 0, errors.New("data return: the package does not name exactly one school or Träger")
	}
	return kind, ids[0], nil
}

// writeZip writes the checked package as one ZIP: manifest.json and
// COMPLETE first, then every listed file in manifest order. It hashes each
// file as it copies it and refuses one that differs from what checkPackage
// verified, so a file swapped or linked after the check never leaves.
func writeZip(w io.Writer, checked checkedPackage, fingerprint func() (io.Writer, func() string)) error {
	archive := zip.NewWriter(w)
	expected := []organizationtenancy.ExportArtifact{{Path: manifestFile, SHA256: checked.manifestSHA256, Bytes: -1},
		{Path: completeFile, SHA256: checked.completeSHA256, Bytes: -1}}
	for _, artifact := range append(expected, checked.artifacts...) {
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: artifact.Path, Method: zip.Deflate})
		if err != nil {
			return err
		}
		if err := copyVerified(entry, checked.dir, artifact, fingerprint); err != nil {
			return err
		}
	}
	return archive.Close()
}

// copyVerified copies one regular file, never through a link, and compares
// its SHA-256 (and size, when known) with the checked value.
func copyVerified(w io.Writer, dir string, artifact organizationtenancy.ExportArtifact, fingerprint func() (io.Writer, func() string)) error {
	full := filepath.Join(dir, filepath.FromSlash(artifact.Path))
	before, err := os.Lstat(full)
	if err != nil || !before.Mode().IsRegular() {
		return errors.New("data return: a package file changed after the check")
	}
	file, err := os.Open(full)
	if err != nil {
		return errors.New("data return: a package file changed after the check")
	}
	defer func() { _ = file.Close() }()
	if opened, err := file.Stat(); err != nil || !os.SameFile(before, opened) {
		return errors.New("data return: a package file changed after the check")
	}
	hash, digest := fingerprint()
	n, err := io.Copy(io.MultiWriter(w, hash), file)
	if err != nil {
		return err
	}
	if digest() != artifact.SHA256 || artifact.Bytes >= 0 && n != artifact.Bytes {
		return errors.New("data return: a package file changed after the check")
	}
	return nil
}
