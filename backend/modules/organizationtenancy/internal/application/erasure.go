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

func NewErasure(store ports.ErasureStore, files organizationtenancy.ErasureFiles) (*Erasure, error) {
	if store == nil || files == nil {
		return nil, errors.New("erasure: database and file store required")
	}
	return &Erasure{store: store, files: files}, nil
}

var orderReferencePattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,39}$`)

func validateErasureRequest(r organizationtenancy.ErasureRequest) error {
	if r.ReceiptID != 0 {
		if r.ReceiptID < 0 || r.SchoolID != 0 || r.OrganizationID != 0 || r.OperatorID != 0 || r.OrderReference != "" {
			return errors.New("erasure: select a positive receipt ID without an erasure target")
		}
		return nil
	}
	if (r.SchoolID > 0) == (r.OrganizationID > 0) || r.SchoolID < 0 || r.OrganizationID < 0 || r.OperatorID <= 0 || !orderReferencePattern.MatchString(r.OrderReference) {
		return errors.New("erasure: select one school or organization, active operator ID and non-personal reference (3-40 uppercase letters/digits/hyphens)")
	}
	if r.LockTimeout < time.Millisecond || r.LockTimeout > time.Minute || r.StatementTimeout <= r.LockTimeout || r.StatementTimeout > time.Hour {
		return errors.New("erasure: require 1ms <= lock timeout <= 1m and lock timeout < statement timeout <= 1h")
	}
	target := fmt.Sprintf("school:%d", r.SchoolID)
	if r.OrganizationID > 0 {
		target = fmt.Sprintf("organization:%d", r.OrganizationID)
	}
	if !r.DryRun && (!r.PrerequisitesChecked || !r.WritersStopped || r.Confirmation != target) {
		return errors.New("erasure: check order, authorization, return/deadlines, stop writers and confirm exact target")
	}
	return nil
}

func erasureFileCatalog() ([]organizationtenancy.LifecycleFileKind, error) {
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	if err != nil {
		return nil, errors.New("erasure: catalog unavailable")
	}
	var catalog organizationtenancy.DataLifecycleCatalog
	if err = json.Unmarshal(raw, &catalog); err != nil {
		return nil, errors.New("erasure: catalog invalid")
	}
	return catalog.Files, validateErasureFiles(catalog.Files)
}

func erasureSchoolIDs(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest) ([]int64, error) {
	if r.OrganizationID == 0 {
		return []int64{r.SchoolID}, nil
	}
	raw, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "schools", r.OrganizationID, r.OperatorID, r.OrderReference)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err = json.Unmarshal(raw, &ids); err != nil {
		return nil, errors.New("erasure: invalid school inventory")
	}
	return ids, nil
}

func (s *Erasure) Run(ctx context.Context, r organizationtenancy.ErasureRequest) ([]json.RawMessage, error) {
	if err := validateErasureRequest(r); err != nil {
		return nil, err
	}
	if r.ReceiptID != 0 {
		proof, err := s.store.Call(ctx, 5*time.Second, 5*time.Minute, "read", r.ReceiptID)
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{proof}, nil
	}
	kinds, err := erasureFileCatalog()
	if err != nil {
		return nil, err
	}
	ids, err := erasureSchoolIDs(ctx, s.store, r)
	if err != nil {
		return nil, err
	}
	var results []json.RawMessage
	for _, id := range ids {
		err = s.store.WithSchoolLock(ctx, id, func(lockCtx context.Context) error {
			proof, err := s.eraseSchool(lockCtx, s.store, r, id, kinds)
			if proof != nil {
				results = append(results, proof)
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
	supported := map[string]string{"files": "tenantDir", "announcement-attachments": "tenantDir", "student-documents": "tenantDir", "staff-documents": "tenantDir", "enrollment-uploads": "tenantDir", "student-photos": "tenantPrefix", "login-images": "tenantPrefix", "enrollment-legal-documents": "tenantPrefix", "enrollment-form-legal-documents": "tenantPrefix", "avatars": "account"}
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
func (s *Erasure) previewSchool(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest, id int64, kinds []organizationtenancy.LifecycleFileKind) (json.RawMessage, error) {
	rows, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "preview", id, r.OperatorID)
	if err != nil {
		return nil, err
	}
	counts, err := s.filePhase(ctx, id, kinds, false)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"school_id": id, "status": "dry_run", "row_counts": rows, "file_counts": counts})
}

func (s *Erasure) prepareSchoolErasure(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest, id int64, kinds []organizationtenancy.LifecycleFileKind) (organizationtenancy.ErasureProof, json.RawMessage, error) {
	var proof organizationtenancy.ErasureProof
	raw, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "begin", id, r.OperatorID, r.OrderReference)
	if err != nil {
		return proof, nil, err
	}
	orderID, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return proof, nil, errors.New("erasure: invalid order ID")
	}
	raw, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "read", orderID)
	if err != nil {
		return proof, nil, err
	}
	if err = json.Unmarshal(raw, &proof); err != nil {
		return proof, nil, errors.New("erasure: invalid receipt")
	}
	if proof.InventoryAt != nil || proof.Status == "completed" {
		return proof, raw, nil
	}
	counts, err := s.filePhase(ctx, id, kinds, false)
	if err != nil {
		return proof, raw, err
	}
	data, err := json.Marshal(counts)
	if err != nil {
		return proof, raw, err
	}
	_, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", orderID, "inventory", string(data))
	return proof, raw, err
}

func eraseSchoolDatabase(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest, id, orderID int64) (json.RawMessage, error) {
	if _, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "erase", id, orderID); err != nil {
		return nil, err
	}
	raw, err := store.Call(ctx, r.LockTimeout, r.StatementTimeout, "read", orderID)
	if err != nil {
		return nil, err
	}
	var proof organizationtenancy.ErasureProof
	if err = json.Unmarshal(raw, &proof); err != nil || proof.DBDoneAt == nil {
		return raw, errors.New("erasure: database deletion failed; retry same order, see receipt SQLSTATE")
	}
	return raw, nil
}

func (s *Erasure) removeAndVerifyFiles(ctx context.Context, id int64, kinds []organizationtenancy.LifecycleFileKind) error {
	if _, err := s.filePhase(ctx, id, kinds, true); err != nil {
		return err
	}
	counts, err := s.filePhase(ctx, id, kinds, false)
	if err != nil {
		return err
	}
	for _, n := range counts {
		if n != 0 {
			return errors.New("erasure: files remain; order is not complete")
		}
	}
	return nil
}

func (s *Erasure) eraseSchool(ctx context.Context, store ports.ErasureStore, r organizationtenancy.ErasureRequest, id int64, kinds []organizationtenancy.LifecycleFileKind) (json.RawMessage, error) {
	if r.DryRun {
		return s.previewSchool(ctx, store, r, id, kinds)
	}
	proof, raw, err := s.prepareSchoolErasure(ctx, store, r, id, kinds)
	if err != nil || proof.Status == "completed" {
		return raw, err
	}
	if proof.DBDoneAt == nil {
		raw, err = eraseSchoolDatabase(ctx, store, r, id, proof.ID)
		if err != nil {
			return raw, err
		}
	}
	if err = s.removeAndVerifyFiles(ctx, id, kinds); err != nil {
		// A cancelled caller cannot report success. Keep the failed marker best-effort
		// and independent of its context; db_done remains a safe restart point.
		markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		failed, markErr := store.Call(markCtx, r.LockTimeout, r.StatementTimeout, "phase", proof.ID, "failed", "{}")
		if markErr != nil {
			return raw, errors.New("erasure: file phase and failure marker failed; order is not complete")
		}
		return failed, err
	}
	if _, err = store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", proof.ID, "files_done", "{}"); err != nil {
		return nil, err
	}
	return store.Call(ctx, r.LockTimeout, r.StatementTimeout, "phase", proof.ID, "completed", "{}")
}
