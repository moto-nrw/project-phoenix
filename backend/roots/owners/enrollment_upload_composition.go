package owners

import (
	"context"
	"errors"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// NewEnrollmentRequestUploads binds the staff reads, the retention and the
// object sweep of the files parents attach (#4783) to Enrollment's records,
// the Audit Platform's access log and the operator-managed retention window.
// A nil access log refuses every open; nil settings fail the retention run.
func NewEnrollmentRequestUploads(records enrollmentOwner.RequestUploadRecords, accessLog enrollmentAccessLogAppender, settings enrollmentOwner.UploadRetentionSettings, logger *slog.Logger) *enrollmentOwner.RequestUploads {
	deps := enrollmentOwner.RequestUploadDependencies{
		Records: records, Settings: settings, Logger: logger,
		NotFound: func(err error) bool { return errors.Is(err, transactionruntime.ErrNoRows) },
	}
	if accessLog != nil {
		deps.AccessLog = enrollmentUploadAccessLog{repo: accessLog}
	}
	return enrollmentOwner.NewRequestUploads(deps)
}

// enrollmentUploadAccessLog appends a staff read of an upload to
// audit.data_access_log. The read concerns one moment, so the range is the
// instant of the access.
type enrollmentUploadAccessLog struct {
	repo enrollmentAccessLogAppender
}

func (l enrollmentUploadAccessLog) RecordUploadAccess(ctx context.Context, entry enrollmentOwner.UploadAccessEntry) error {
	at := entry.AccessedAt
	if at.IsZero() {
		at = time.Now()
	}
	return l.repo.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: entry.ActorAccountID,
		ActorRole:      entry.ActorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentUploadView,
		RangeStart:     at,
		RangeEnd:       at,
		AccessedAt:     at,
		Metadata: map[string]any{
			"request_id": entry.RequestID,
			"upload_id":  entry.UploadID,
			"field_key":  entry.FieldKey,
		},
	})
}

// enrollmentUploadRetentionStep deletes the expired uploads of one school in
// the nightly cleanup (#4783); the file store sweep removes their objects.
func enrollmentUploadRetentionStep(settings enrollmentOwner.UploadRetentionSettings, logger *slog.Logger) schedulerPorts.RetentionStep {
	uploads := NewEnrollmentRequestUploads(enrollmentCompose.New(), nil, settings, logger.With("service", "enrollment-upload-retention"))
	return ownerRetentionStep(auditModels.DeletionTypeEnrollmentUploadRetention, uploads.DeleteExpiredUploads)
}
