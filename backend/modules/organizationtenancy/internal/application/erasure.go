package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

type Erasure struct {
	store ports.ErasureStore
	files organizationtenancy.ErasureFiles
}

func NewErasure(store ports.ErasureStore, files organizationtenancy.ErasureFiles) *Erasure {
	return &Erasure{store: store, files: files}
}

var orderReferencePattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,39}$`)

func (s *Erasure) Run(ctx context.Context, r organizationtenancy.ErasureRequest) ([]json.RawMessage, error) {
	if r.ReceiptID != 0 {
		if r.ReceiptID < 0 || r.SchoolID != 0 || r.OrganizationID != 0 || r.OperatorID != 0 || r.OrderReference != "" {
			return nil, errors.New("erasure: select a positive receipt ID without an erasure target")
		}
		proof, err := s.store.Call(ctx, 5*time.Second, 5*time.Minute, "read", r.ReceiptID)
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{proof}, nil
	}

	if (r.SchoolID > 0) == (r.OrganizationID > 0) || r.SchoolID < 0 || r.OrganizationID < 0 || r.OperatorID <= 0 || !orderReferencePattern.MatchString(r.OrderReference) {
		return nil, errors.New("erasure: select one school or organization, active operator ID and non-personal reference (3-40 uppercase letters/digits/hyphens)")
	}
	if r.LockTimeout < time.Millisecond || r.LockTimeout > time.Minute || r.StatementTimeout <= r.LockTimeout || r.StatementTimeout > time.Hour {
		return nil, errors.New("erasure: require 1ms <= lock timeout <= 1m and lock timeout < statement timeout <= 1h")
	}
	target := fmt.Sprintf("school:%d", r.SchoolID)
	if r.OrganizationID > 0 {
		target = fmt.Sprintf("organization:%d", r.OrganizationID)
	}
	if !r.DryRun && (!r.PrerequisitesChecked || !r.WritersStopped || r.Confirmation != target) {
		return nil, errors.New("erasure: check order, authorization, return/retention deadlines, stop writers and confirm exact target")
	}
	data, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	if err != nil {
		return nil, errors.New("erasure: catalog unavailable")
	}
	var catalog organizationtenancy.DataLifecycleCatalog
	if err = json.Unmarshal(data, &catalog); err != nil {
		return nil, errors.New("erasure: catalog invalid")
	}
	if err = validateErasureFiles(catalog.Files); err != nil {
		return nil, err
	}
	store := s.store
	schools := []int64{r.SchoolID}
	if r.OrganizationID > 0 {
		data, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "schools", r.OrganizationID, r.OperatorID, r.OrderReference)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &schools); err != nil {
			return nil, errors.New("erasure: invalid school inventory")
		}
	}
	var results []json.RawMessage
	for _, id := range schools {
		err := store.WithSchoolLock(ctx, id, func() error {
			result, err := s.eraseSchool(ctx, store, r, id, catalog.Files)
			if result != nil {
				results = append(results, result)
			}
			return err
		})
		if err != nil {
			return results, err
		}
	}
	if r.OrganizationID > 0 && !r.DryRun {
		return results, errors.New("erasure: school phases finished; organization root deletion unsupported by DB contract, organization order remains open")
	}
	return results, nil
}

// A new namespace or changed layout needs explicit review, not an automatic
// destructive interpretation. Avatars retain global objects but erase legacy
// school directories. This is an execution guard, not another data catalog.
func validateErasureFiles(kinds []organizationtenancy.LifecycleFileKind) error {
	supported := map[string]string{"files": "tenantDir", "announcement-attachments": "tenantDir", "student-documents": "tenantDir", "staff-documents": "tenantDir", "student-photos": "tenantPrefix", "login-images": "tenantPrefix", "enrollment-legal-documents": "tenantPrefix", "enrollment-form-legal-documents": "tenantPrefix", "avatars": "account"}
	if len(kinds) != len(supported) {
		return errors.New("erasure: file catalog needs reviewed deletion paths")
	}
	for _, kind := range kinds {
		layout, ok := supported[kind.Kind]
		if !ok || layout != kind.Layout {
			return errors.New("erasure: unsupported file deletion path")
		}
		delete(supported, kind.Kind)
	}
	return nil
}
func (s *Erasure) filePhase(ctx context.Context, id int64, kinds []organizationtenancy.LifecycleFileKind, remove bool) (map[string]int64, error) {
	counts := make(map[string]int64, len(kinds))
	for _, kind := range kinds {
		layout := kind.Layout
		if kind.Kind == "avatars" {
			layout = "tenantDir"
		}
		n, err := s.files.TenantObjects(ctx, kind.Kind, layout, id, remove)
		if err != nil {
			return nil, errors.New("erasure: file phase failed; no overall completion, retry same order after fixing storage")
		}
		counts[kind.Kind] = n
	}
	return counts, nil
}
func (s *Erasure) eraseSchool(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest, id int64, kinds []organizationtenancy.LifecycleFileKind) (json.RawMessage, error) {
	if r.DryRun {
		rows, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "preview", id, r.OperatorID)
		if err != nil {
			return nil, err
		}
		counts, err := s.filePhase(ctx, id, kinds, false)
		if err != nil {
			return nil, err
		}
		result, err := json.Marshal(map[string]any{"school_id": id, "status": "dry_run", "row_counts": rows, "file_counts": counts})
		return result, err
	}
	raw, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "begin", id, r.OperatorID, r.OrderReference)
	if err != nil {
		return nil, err
	}
	orderID, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return nil, errors.New("erasure: invalid order ID")
	}
	read := func() (json.RawMessage, error) {
		return store.Call(ctx, r.LockTimeout, r.StatementTimeout, "read", orderID)
	}
	raw, err = read()
	if err != nil {
		return nil, err
	}
	var proof organizationtenancy.ErasureProof
	if err = json.Unmarshal(raw, &proof); err != nil {
		return nil, errors.New("erasure: invalid receipt")
	}
	if proof.Status == "completed" {
		return raw, nil
	}
	if proof.InventoryAt == nil {
		counts, err := s.filePhase(ctx, id, kinds, false)
		if err != nil {
			return raw, err
		}
		data, err := json.Marshal(counts)
		if err != nil {
			return raw, err
		}
		if _, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", orderID, "inventory", string(data)); err != nil {
			return raw, err
		}
	}
	if proof.DBDoneAt == nil {
		if _, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "erase", id, orderID); err != nil {
			return nil, err
		}
		raw, err = read()
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &proof); err != nil || proof.DBDoneAt == nil {
			return raw, errors.New("erasure: database deletion failed; retry same order, see receipt SQLSTATE")
		}
	}
	if _, err = s.filePhase(ctx, id, kinds, true); err != nil {
		// Cancellation must not turn into success; a best-effort failed marker is
		// independent of the cancelled caller. db_done still makes retries safe.
		markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		failed, markErr := store.Call(markCtx, r.LockTimeout, r.StatementTimeout, "phase", orderID, "failed", "{}")
		if markErr != nil {
			return raw, errors.New("erasure: file phase and failure marker failed; order is not complete")
		}
		return failed, err
	}
	// Verify all namespaces again before recording any completion.
	counts, err := s.filePhase(ctx, id, kinds, false)
	if err != nil {
		return nil, err
	}
	for _, n := range counts {
		if n != 0 {
			return nil, errors.New("erasure: files remain; order is not complete")
		}
	}
	if _, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", orderID, "files_done", "{}"); err != nil {
		return nil, err
	}
	return store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", orderID, "completed", "{}")
}
