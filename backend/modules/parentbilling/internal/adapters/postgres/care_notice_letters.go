package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The stored originals of the Jahresbescheide and their sends (#6347). The
// request role reads and adds them and never changes them (triggers of
// 1.15.877); every statement names the Träger in its own filter.

type careNoticeLetterRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_notice_letters"`

	ID             int64     `bun:"id,pk,nullzero"`
	OrganizationID int64     `bun:"organization_id"`
	RunID          int64     `bun:"run_id"`
	NoticeID       int64     `bun:"notice_id"`
	Filename       string    `bun:"filename"`
	PDF            []byte    `bun:"pdf"`
	SHA256         string    `bun:"sha256,scanonly"`
	CreatedAt      time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r careNoticeLetterRow) record(withFile bool) parentbilling.CareNoticeLetterRecord {
	record := parentbilling.CareNoticeLetterRecord{ID: r.ID, NoticeID: r.NoticeID, SHA256: r.SHA256, CreatedAt: r.CreatedAt}
	if withFile {
		record.File = &parentbilling.BillingLetterFile{Filename: r.Filename, ContentType: "application/pdf", Data: r.PDF}
	}
	return record
}

type careNoticeLetterMailingRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_notice_letter_mailings"`

	ID                   int64     `bun:"id,pk,autoincrement"`
	OrganizationID       int64     `bun:"organization_id"`
	RunID                int64     `bun:"run_id"`
	LetterID             int64     `bun:"letter_id"`
	DeliveryID           int64     `bun:"delivery_id"`
	RecipientEmail       string    `bun:"recipient_email"`
	RequestedByAccountID *int64    `bun:"requested_by_account_id"`
	CreatedAt            time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r careNoticeLetterMailingRow) send() parentbilling.CareNoticeLetterSend {
	return parentbilling.CareNoticeLetterSend{LetterID: r.LetterID, DeliveryID: r.DeliveryID, RecipientEmail: r.RecipientEmail,
		CreatedAt: r.CreatedAt}
}

// careNoticeLetterNoticeConstraint keeps one stored original per notice.
const careNoticeLetterNoticeConstraint = "uq_care_notice_letters_notice"

// InsertCareNoticeLetter stores the PDF of one notice and returns it with
// the SHA-256 the database computed. A second PDF of the same notice is
// ErrBillingLettersExist: the first one stays the original.
func (s *Store) InsertCareNoticeLetter(ctx context.Context, organizationID, runID, noticeID int64, file parentbilling.BillingLetterFile) (parentbilling.CareNoticeLetterRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeLetterRecord{}, err
	}
	row := &careNoticeLetterRow{OrganizationID: organizationID, RunID: runID, NoticeID: noticeID, Filename: file.Filename, PDF: file.Data}
	if _, err := db.NewInsert().Model(row).Returning("id, sha256, created_at").Exec(ctx); err != nil {
		if constraintOf(err) == careNoticeLetterNoticeConstraint {
			return parentbilling.CareNoticeLetterRecord{}, parentbilling.ErrBillingLettersExist
		}
		return parentbilling.CareNoticeLetterRecord{}, fmt.Errorf("insert care notice letter: %w", err)
	}
	return row.record(false), nil
}

// CareNoticeLetterRecords reads the stored originals of a run, with their
// PDF when withFiles is set.
func (s *Store) CareNoticeLetterRecords(ctx context.Context, organizationID, runID int64, withFiles bool) ([]parentbilling.CareNoticeLetterRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careNoticeLetterRow
	query := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).OrderExpr("id")
	if withFiles {
		query = query.Column("care_notice_letter_row.*")
	} else {
		query = query.Column("id", "organization_id", "run_id", "notice_id", "filename", "sha256", "created_at")
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care notice letters: %w", err)
	}
	records := make([]parentbilling.CareNoticeLetterRecord, len(rows))
	for i, row := range rows {
		records[i] = row.record(withFiles)
	}
	return records, nil
}

// CareNoticeLetterRecord reads the stored original of one notice of a run.
func (s *Store) CareNoticeLetterRecord(ctx context.Context, organizationID, runID, noticeID int64) (parentbilling.CareNoticeLetterRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeLetterRecord{}, err
	}
	var row careNoticeLetterRow
	if err := db.NewSelect().Model(&row).Column("care_notice_letter_row.*").Where("organization_id = ?", organizationID).
		Where("run_id = ?", runID).Where("notice_id = ?", noticeID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareNoticeLetterRecord{}, parentbilling.ErrBillingLetterNotFound
		}
		return parentbilling.CareNoticeLetterRecord{}, fmt.Errorf("read care notice letter: %w", err)
	}
	return row.record(true), nil
}

// CareNoticeLetterMailings reads every send of the originals of a run in the
// order they were stored.
func (s *Store) CareNoticeLetterMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareNoticeLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careNoticeLetterMailingRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care notice mails: %w", err)
	}
	sends := make([]parentbilling.CareNoticeLetterSend, len(rows))
	for i, row := range rows {
		sends[i] = row.send()
	}
	return sends, nil
}

// InsertCareNoticeLetterMailing stores one send of an original.
func (s *Store) InsertCareNoticeLetterMailing(ctx context.Context, organizationID, runID, accountID int64,
	send parentbilling.CareNoticeLetterSend,
) (parentbilling.CareNoticeLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeLetterSend{}, err
	}
	row := &careNoticeLetterMailingRow{OrganizationID: organizationID, RunID: runID, LetterID: send.LetterID,
		DeliveryID: send.DeliveryID, RecipientEmail: send.RecipientEmail, RequestedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return parentbilling.CareNoticeLetterSend{}, fmt.Errorf("insert care notice mail: %w", err)
	}
	return row.send(), nil
}
