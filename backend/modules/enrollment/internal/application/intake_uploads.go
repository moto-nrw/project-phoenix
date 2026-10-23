package application

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Upload thresholds per IP. A family uploads a handful of proofs, maybe
// again after a typo; the public form's submission bucket (10 per hour)
// would refuse a parent who replaces a blurred photo twice.
const (
	rateLimitWindowUploadIP      = time.Hour
	rateLimitMaxAttemptsUploadIP = 30
)

// IntakeUploads stores the files parents attach to a request (#4783).
type IntakeUploads = enrollmentNative.RequestUploadRecords

var _ enrollmentNative.UploadIntake = (*Intake)(nil)

// PrepareUpload checks that the phase's form takes the file in the named
// field, counts the attempt and stores the unclaimed row with the hash of a
// fresh token. It runs in the caller's tenant transaction, which the root
// commits before the HTTP adapter writes the bytes: a crash in between leaves
// an expiring row, never bytes no row points at.
func (s *Intake) PrepareUpload(ctx context.Context, input enrollmentNative.UploadInput) (*enrollmentNative.PreparedUpload, error) {
	if s.deps.Uploads == nil || s.deps.Fingerprint == nil || s.deps.Random == nil {
		return nil, errors.New("enrollment uploads are not configured")
	}
	return s.prepareUpload(ctx, input)
}

func (s *Intake) prepareUpload(ctx context.Context, input enrollmentNative.UploadInput) (*enrollmentNative.PreparedUpload, error) {
	field, err := s.uploadField(ctx, input)
	if err != nil {
		return nil, err
	}
	options := field.FileOptions
	if input.SizeBytes <= 0 || input.SizeBytes > options.MaxBytes() || !options.AllowsContentType(input.ContentType) {
		return nil, &enrollmentNative.UploadRejection{MaxMB: options.MaxSizeMB, AllowedTypes: options.AllowedTypes}
	}
	if err := s.enforceUploadRateLimit(ctx, input); err != nil {
		return nil, err
	}
	token, err := s.newStatusToken()
	if err != nil {
		return nil, fmt.Errorf("upload: generate token: %w", err)
	}
	storedName, err := s.newUploadStoredName(input.Extension)
	if err != nil {
		return nil, fmt.Errorf("upload: stored name: %w", err)
	}
	upload := &enrollmentNative.RequestUpload{
		TenantID: input.TenantID, PhaseID: input.PhaseID, FieldKey: field.Key,
		Filename: uploadDisplayName(input.Filename, input.Extension), StoredName: storedName,
		SizeBytes: input.SizeBytes, ContentType: input.ContentType, TokenHash: s.uploadTokenHash(token),
	}
	if err := s.deps.Uploads.InsertRequestUpload(ctx, upload); err != nil {
		return nil, fmt.Errorf("upload: store row: %w", err)
	}
	return &enrollmentNative.PreparedUpload{Upload: upload, Token: token}, nil
}

// uploadField resolves the file field of the phase's pinned form. Any phase
// of the school may take uploads: an edit through the status link runs after
// the window closed, and an unclaimed upload expires on its own.
func (s *Intake) uploadField(ctx context.Context, input enrollmentNative.UploadInput) (*enrollmentNative.FormField, error) {
	phase, err := s.loadPhaseForSubmission(ctx, input.PhaseID)
	if err != nil || phase.TenantID != input.TenantID {
		return nil, enrollmentNative.ErrUploadFieldNotFound
	}
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return nil, fmt.Errorf("upload: load schema: %w", err)
	}
	field := enrollmentNative.FileFieldByKey(schema, strings.TrimSpace(input.FieldKey))
	if field == nil || field.FileOptions == nil {
		return nil, enrollmentNative.ErrUploadFieldNotFound
	}
	return field, nil
}

// enforceUploadRateLimit counts the upload in the IP's upload bucket through
// the submission limiter's own transaction, so a refused upload still counts.
func (s *Intake) enforceUploadRateLimit(ctx context.Context, input enrollmentNative.UploadInput) error {
	if strings.TrimSpace(input.RemoteIP) == "" {
		return nil
	}
	err := s.enforceRateLimit(ctx, SubmitRequest{uploadAttempt: true, TenantID: input.TenantID, RemoteIP: input.RemoteIP})
	if errors.Is(err, enrollmentNative.ErrRateLimited) {
		return enrollmentNative.ErrUploadRateLimited
	}
	return err
}

// countUploadAttempt is the upload bucket of enforceRateLimitBuckets.
func (s *Intake) countUploadAttempt(ctx context.Context, req SubmitRequest) error {
	return s.countRateLimitAttempt(ctx, req.TenantID, rateLimitBucket{
		keyType: enrollmentNative.SubmissionRateLimitKeyTypeUploadIP, label: "upload IP",
		window: rateLimitWindowUploadIP, maxAttempts: rateLimitMaxAttemptsUploadIP,
	}, strings.TrimSpace(req.RemoteIP))
}

// newUploadStoredName is a random object name with the detected extension.
// The display name never reaches the store: random hex cannot collide in
// practice, carries no traversal payload and leaks nothing.
func (s *Intake) newUploadStoredName(extension string) (string, error) {
	if !strings.HasPrefix(extension, ".") || strings.ContainsAny(extension, "/\\") {
		return "", fmt.Errorf("unsupported extension %q", extension)
	}
	buf := make([]byte, 16)
	if err := s.deps.Random(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + extension, nil
}

func (s *Intake) uploadTokenHash(token string) string {
	return s.deps.Fingerprint([]byte(strings.TrimSpace(token)))
}

// uploadDisplayName keeps the parent's file name for the staff view, bounded
// and with the extension the content was detected as.
func uploadDisplayName(name, extension string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "/", "_"), "\\", "_"))
	if runes := []rune(name); len(runes) > 120 {
		name = string(runes[:120])
	}
	if name == "" {
		name = "Nachweis" + extension
	}
	return name
}

// fileClaims are the uploads a write claims for its request.
type fileClaims struct {
	uploadIDs []int64
}

// resolveFileAnswers turns the file answers of the guardian answers into
// their stored form, in place. A fresh item must carry the token of an
// unclaimed upload of the same phase field; a kept item must name an upload
// the request already holds in that field. The metadata comes from the row,
// never from the client. Uploads of the request no longer named are
// deleted, which queues their objects.
func (s *Intake) resolveFileAnswers(ctx context.Context, phaseID int64, schema *enrollmentNative.FormSchema, requestID int64, answers map[string]any) (*fileClaims, error) {
	keys := enrollmentNative.FileFieldKeys(schema)
	if len(keys) == 0 || s.deps.Uploads == nil {
		return &fileClaims{}, nil
	}
	held, err := s.heldUploads(ctx, requestID)
	if err != nil {
		return nil, err
	}
	claims := &fileClaims{}
	kept := map[int64]bool{}
	for _, key := range keys {
		field := enrollmentNative.FileFieldByKey(schema, key)
		raw, ok := answers[key]
		if !ok {
			continue
		}
		stored, err := s.resolveFieldAnswer(ctx, phaseID, field, raw, held, kept, claims)
		if err != nil {
			return nil, err
		}
		if len(stored) == 0 {
			delete(answers, key)
			continue
		}
		answers[key] = stored
	}
	if requestID > 0 {
		if err := s.deps.Uploads.DeleteRequestUploads(ctx, requestID, droppedUploadIDs(held, kept)); err != nil {
			return nil, fmt.Errorf("delete removed uploads: %w", err)
		}
	}
	return claims, nil
}

func (s *Intake) heldUploads(ctx context.Context, requestID int64) (map[int64]*enrollmentNative.RequestUpload, error) {
	held := map[int64]*enrollmentNative.RequestUpload{}
	if requestID <= 0 {
		return held, nil
	}
	rows, err := s.deps.Uploads.RequestUploads(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("load request uploads: %w", err)
	}
	for _, row := range rows {
		held[row.ID] = row
	}
	return held, nil
}

func (s *Intake) resolveFieldAnswer(ctx context.Context, phaseID int64, field *enrollmentNative.FormField, raw any, held map[int64]*enrollmentNative.RequestUpload, kept map[int64]bool, claims *fileClaims) ([]enrollmentNative.FileAnswer, error) {
	items, err := decodeFileAnswer(raw)
	if err != nil || len(items) > field.FileOptions.MaxFiles() {
		return nil, uploadExpired(field.Key, "file answer is malformed or has too many files")
	}
	stored := make([]enrollmentNative.FileAnswer, 0, len(items))
	var hashes []string
	for _, item := range items {
		if token := strings.TrimSpace(item.Token); token != "" {
			hashes = append(hashes, s.uploadTokenHash(token))
			continue
		}
		id, parseErr := strconv.ParseInt(item.UploadID, 10, 64)
		upload := held[id]
		if parseErr != nil || upload == nil || upload.FieldKey != field.Key || kept[id] {
			return nil, uploadExpired(field.Key, "file answer names an upload the request does not hold")
		}
		kept[id] = true
		stored = append(stored, upload.Answer())
	}
	if len(hashes) == 0 {
		return stored, nil
	}
	fresh, err := s.deps.Uploads.UnclaimedRequestUploads(ctx, phaseID, field.Key, hashes, time.Now().Add(-enrollmentNative.UnclaimedUploadLifetime))
	if err != nil {
		return nil, fmt.Errorf("load unclaimed uploads: %w", err)
	}
	if len(fresh) != len(hashes) {
		return nil, uploadExpired(field.Key, "an upload token is unknown, expired or already used")
	}
	for _, upload := range fresh {
		claims.uploadIDs = append(claims.uploadIDs, upload.ID)
		stored = append(stored, upload.Answer())
	}
	return stored, nil
}

// claim binds the resolved fresh uploads to the request.
func (s *Intake) claimFileUploads(ctx context.Context, claims *fileClaims, requestID int64) error {
	if claims == nil || len(claims.uploadIDs) == 0 {
		return nil
	}
	if err := s.deps.Uploads.ClaimRequestUploads(ctx, claims.uploadIDs, requestID, time.Now()); err != nil {
		return fmt.Errorf("claim uploads: %w", err)
	}
	s.logger().Info("enrollment uploads claimed",
		slog.Int64("request_id", requestID),
		slog.Int("uploads", len(claims.uploadIDs)))
	return nil
}

func droppedUploadIDs(held map[int64]*enrollmentNative.RequestUpload, kept map[int64]bool) []int64 {
	var ids []int64
	for id := range held {
		if !kept[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func uploadExpired(key, detail string) error {
	return enrollmentNative.InvalidInput(enrollmentNative.CodeUploadExpired, "custom_data."+key, fmt.Errorf("file field %q: %s", key, detail))
}

// decodeFileAnswer reads a file answer: a list of items, nil or empty for
// no file.
func decodeFileAnswer(raw any) ([]enrollmentNative.FileAnswer, error) {
	if raw == nil {
		return nil, nil
	}
	var items []enrollmentNative.FileAnswer
	if err := decodeStructured(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// fileAnswerPresent reports whether a required file field holds a file.
func fileAnswerPresent(raw any) bool {
	items, err := decodeFileAnswer(raw)
	return err == nil && len(items) > 0
}
