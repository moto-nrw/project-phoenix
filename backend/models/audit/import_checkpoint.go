package audit

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ImportCheckpoint is an immutable receipt appended with a committed import
// batch. Payload contains the batch's outcomes, not the uploaded file. The
// workflow owns its encoding and rehydrates row data from the replay request.
type ImportCheckpoint struct {
	ImportKey string          `json:"import_key"`
	LastRow   int             `json:"last_row"`
	TotalRows int             `json:"total_rows"`
	Payload   json.RawMessage `json:"payload"`
}

// ImportCheckpointReader reads receipts only for the tenant in the active
// transaction. ImportKey binds the upload, options and importing identity.
type ImportCheckpointReader interface {
	ListImportCheckpoints(context.Context, string) ([]ImportCheckpoint, error)
}

func (c ImportCheckpoint) Validate() error {
	key, err := hex.DecodeString(c.ImportKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("import checkpoint requires a SHA-256 key")
	}
	if c.LastRow <= 0 || c.TotalRows < c.LastRow {
		return fmt.Errorf("import checkpoint row position is invalid")
	}
	if !json.Valid(c.Payload) {
		return fmt.Errorf("import checkpoint payload is invalid JSON")
	}
	return nil
}

// AttachImportCheckpoint attaches the receipt to the batch's existing GDPR
// record, so neither can commit without the other. Audit remains append-only.
func (d *DataImport) AttachImportCheckpoint(checkpoint ImportCheckpoint) error {
	if err := checkpoint.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("encode import checkpoint: %w", err)
	}
	if d.Metadata == nil {
		d.Metadata = JSONBMap{}
	}
	d.Metadata["import_checkpoint"] = json.RawMessage(encoded)
	return nil
}
