package tenantexport

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The Löschkonzept (§5) asks the return to hand over the documents
// themselves: letters, fee notices, every contract version, paper contracts,
// signature images, proofs. The rows carry them only as hex, so a Träger
// package also writes each stored document as a file (ADR 0187). Which
// columns are documents follows from the catalog, not from a list here:
// every binary column the catalog exports from a table of the Träger. A
// binary column that is no document is a secret the catalog excludes.

// documentDirectory holds the stored documents of a Träger package.
const documentDirectory = "DOKUMENTE"

const documentIndex = documentDirectory + "/index.jsonl"

const documentNotice = "Gespeicherte Dokumente der Tabellen dieses Trägers als Dateien (Schreiben, Bescheide, Vertragsfassungen, Papierverträge, Unterschriften, Nachweise); index.jsonl nennt je Datei Tabelle, Zeile, Spalte, SHA-256 und Größe."

const documentReadme = `Dieser Ordner enthält die bei moto gespeicherten Dokumente dieses Trägers
als einzelne Dateien: Schreiben und Bestätigungen, Bescheide, alle Fassungen
der Betreuungsverträge, Papierverträge, Unterschriftsbilder, hochgeladene
Nachweise und weitere Dateien des Trägers.

Jede Datei liegt unter <Tabelle>/<Zeilen-ID>-<Spalte>.<Endung>.
index.jsonl nennt je Datei Tabelle, Zeile (row_id und row_key), Spalte,
Medientyp, SHA-256 und Größe in Bytes. Dieselbe Zeile steht in
<Tabelle>.jsonl; dort steht das Dokument zusätzlich als Hex-Wert.
`

// documentRowID is the part of a file name that names its row.
var documentRowID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type documentIndexEntry struct {
	Path      string                     `json:"path"`
	Table     string                     `json:"table"`
	Column    string                     `json:"column"`
	RowID     string                     `json:"row_id"`
	RowKey    map[string]json.RawMessage `json:"row_key"`
	MediaType string                     `json:"media_type"`
	SHA256    string                     `json:"sha256"`
	Bytes     int64                      `json:"bytes"`
}

// documentColumnsOf answers the binary columns a table exports.
func documentColumnsOf(model organizationtenancy.ExportModelTable) []string {
	var columns []string
	for _, column := range model.Columns {
		if column.Type == "bytea" {
			columns = append(columns, column.Name)
		}
	}
	return columns
}

// writeDocumentFiles writes every stored document of the exported tables
// from their table files, after the snapshot: the files hold exactly the
// rows of the package. A document it cannot write fails the package.
func (r *exportRun) writeDocumentFiles(ctx context.Context) error {
	if err := os.Mkdir(filepath.Join(r.options.Output, documentDirectory), 0o700); err != nil {
		return errors.New("tenant export: cannot create the document directory")
	}
	index, err := os.OpenFile(filepath.Join(r.options.Output, filepath.FromSlash(documentIndex)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("tenant export: cannot create the document index")
	}
	defer func() { _ = index.Close() }()
	summary := &organizationtenancy.ExportDocumentFiles{Directory: documentDirectory, Index: documentIndex, Notice: documentNotice,
		Columns: []organizationtenancy.ExportDocumentColumn{}}
	encoder := json.NewEncoder(index)
	for _, model := range r.models {
		columns := documentColumnsOf(model)
		if len(columns) == 0 {
			continue
		}
		if err := r.writeTableDocuments(ctx, model.Name, columns, encoder, summary); err != nil {
			return err
		}
	}
	if index.Sync() != nil || index.Close() != nil {
		return errors.New("tenant export: cannot persist the document index")
	}
	if err := r.recordArtifact(documentIndex); err != nil {
		return err
	}
	if err := r.writeArtifact(path.Join(documentDirectory, "LIESMICH.txt"), []byte(documentReadme)); err != nil {
		return err
	}
	r.manifest.DocumentFiles = summary
	return nil
}

func (r *exportRun) writeTableDocuments(ctx context.Context, table string, columns []string, index *json.Encoder,
	summary *organizationtenancy.ExportDocumentFiles,
) error {
	keys := r.schema.primaryKeys[table]
	if len(keys) == 0 {
		return errors.New("tenant export: the documents of " + table + " have no row to name them by")
	}
	if err := os.Mkdir(filepath.Join(r.options.Output, documentDirectory, table), 0o700); err != nil {
		return errors.New("tenant export: cannot create the document directory of " + table)
	}
	first := len(summary.Columns)
	for _, column := range columns {
		summary.Columns = append(summary.Columns, organizationtenancy.ExportDocumentColumn{Table: table, Column: column})
	}
	return scanExportRows(ctx, filepath.Join(r.options.Output, table+".jsonl"), func(row map[string]json.RawMessage) error {
		rowID, rowKey, err := documentRow(table, keys, row)
		if err != nil {
			return err
		}
		for i, column := range columns {
			entry, err := r.writeDocument(table, column, rowID, row)
			if err != nil || entry == nil {
				return err
			}
			entry.RowKey = rowKey
			if index.Encode(entry) != nil {
				return errors.New("tenant export: cannot write the document index")
			}
			summary.Columns[first+i].Files++
			summary.Columns[first+i].Bytes += entry.Bytes
		}
		return nil
	})
}

// documentRow names a row by its primary key, joined with "_".
func documentRow(table string, keys []string, row map[string]json.RawMessage) (string, map[string]json.RawMessage, error) {
	parts := make([]string, 0, len(keys))
	rowKey := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		raw := row[key]
		var value any
		if json.Unmarshal(raw, &value) != nil {
			return "", nil, exportTableError(table)
		}
		var part string
		switch value.(type) {
		case string:
			_ = json.Unmarshal(raw, &part)
		case float64:
			part = string(raw)
		}
		if !documentRowID.MatchString(part) {
			return "", nil, errors.New("tenant export: a document of " + table + " has no row to name it by")
		}
		parts = append(parts, part)
		rowKey[key] = raw
	}
	return strings.Join(parts, "_"), rowKey, nil
}

// writeDocument writes one stored document; a null column has none.
func (r *exportRun) writeDocument(table, column, rowID string, row map[string]json.RawMessage) (*documentIndexEntry, error) {
	var encoded *string
	if err := json.Unmarshal(row[column], &encoded); err != nil {
		return nil, exportTableError(table)
	}
	if encoded == nil {
		return nil, nil
	}
	// row_to_json renders bytea in PostgreSQL's hex format.
	if !strings.HasPrefix(*encoded, `\x`) {
		return nil, exportTableError(table)
	}
	data, err := hex.DecodeString(strings.TrimPrefix(*encoded, `\x`))
	if err != nil {
		return nil, exportTableError(table)
	}
	mediaType := documentMediaType(column, row, data)
	name := path.Join(documentDirectory, table, rowID+"-"+column+documentExtension(mediaType))
	if err := r.writeArtifact(name, data); err != nil {
		return nil, err
	}
	artifact := r.manifest.Artifacts[len(r.manifest.Artifacts)-1]
	return &documentIndexEntry{Path: name, Table: table, Column: column, RowID: rowID, MediaType: mediaType,
		SHA256: artifact.SHA256, Bytes: artifact.Bytes}, nil
}

// documentMediaType takes the media type the row stores beside the
// document, else the one its content shows.
func documentMediaType(column string, row map[string]json.RawMessage, data []byte) string {
	for _, sibling := range []string{column + "_content_type", "content_type"} {
		var stored string
		if json.Unmarshal(row[sibling], &stored) != nil || stored == "" {
			continue
		}
		if mediaType, _, err := mime.ParseMediaType(stored); err == nil && mediaType != "application/octet-stream" {
			return mediaType
		}
	}
	return sniffDocumentMediaType(data)
}

// sniffDocumentMediaType knows the signatures of the stored document kinds.
func sniffDocumentMediaType(data []byte) string {
	for _, signature := range []struct {
		prefix    string
		mediaType string
	}{
		{"%PDF-", "application/pdf"},
		{"\x89PNG\r\n\x1a\n", "image/png"},
		{"\xff\xd8\xff", "image/jpeg"},
		{"GIF87a", "image/gif"},
		{"GIF89a", "image/gif"},
	} {
		if bytes.HasPrefix(data, []byte(signature.prefix)) {
			return signature.mediaType
		}
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "application/octet-stream"
}

// documentExtension is a fixed mapping, independent of the host's MIME tables.
func documentExtension(mediaType string) string {
	switch mediaType {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	case "text/plain":
		return ".txt"
	case "text/xml", "application/xml":
		return ".xml"
	default:
		return ".bin"
	}
}

// scanExportRows visits each row of a table file of this package.
func scanExportRows(ctx context.Context, name string, visit func(map[string]json.RawMessage) error) error {
	f, err := os.Open(name)
	if err != nil {
		return errors.New("tenant export: table artifact unavailable")
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	for {
		if err := ctx.Err(); err != nil {
			return errors.New("tenant export: interrupted")
		}
		var fields map[string]json.RawMessage
		if err := decoder.Decode(&fields); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return errors.New("tenant export: invalid table artifact")
		}
		if err := visit(fields); err != nil {
			return err
		}
	}
}
