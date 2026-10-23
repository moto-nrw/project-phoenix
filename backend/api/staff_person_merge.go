package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge"
	mergeCompose "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/compose"
	mergeHTTP "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/http"
)

type staffPersonMergeAudit struct {
	create func(context.Context, *auditRecords.OperatorAuditEntry) error
}

func (a staffPersonMergeAudit) RecordStaffPersonMergePreview(ctx context.Context, operatorID, schoolID int64, clientIP net.IP) error {
	changes, err := json.Marshal(struct {
		SchoolID int64  `json:"school_id"`
		View     string `json:"view"`
	}{schoolID, "staff_person_merge_candidates"})
	if err != nil {
		return err
	}
	return a.create(ctx, &auditRecords.OperatorAuditEntry{OperatorID: operatorID, Action: "view", ResourceType: "person", Changes: changes, RequestIP: clientIP})
}

func (a staffPersonMergeAudit) RecordStaffPersonMerge(ctx context.Context, operatorID, schoolID int64, candidate staffpersonmerge.Candidate) error {
	changes, err := json.Marshal(struct {
		SchoolID        int64 `json:"school_id"`
		DonorID         int64 `json:"donor_person_id"`
		KeeperID        int64 `json:"keeper_person_id"`
		DonorStaffID    int64 `json:"donor_staff_id"`
		KeeperStaffID   int64 `json:"keeper_staff_id"`
		SchoolConfirmed bool  `json:"school_confirmed"`
	}{schoolID, candidate.Donor.Person.ID, candidate.Keeper.Person.ID, candidate.Donor.Employment.ID, candidate.Keeper.Employment.ID, true})
	if err != nil {
		return err
	}
	id := candidate.Keeper.Person.ID
	return a.create(ctx, &auditRecords.OperatorAuditEntry{OperatorID: operatorID, Action: "merge", ResourceType: "person", ResourceID: &id, Changes: changes})
}
func newStaffPersonMergeRoutes(db transactionruntime.Database) (http.Handler, error) {
	audit, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	if err != nil {
		return nil, err
	}
	workflow, err := mergeCompose.New(peopleCompose.NewStaffPersonMerge(), membershipCompose.NewStaffPersonMerge(), staffPersonMergeAudit{create: audit.OperatorAuditLog.Create}, operatorIDFromSession)
	if err != nil {
		return nil, err
	}
	return mergeHTTP.Router(workflow, TenantScopeMiddleware), nil
}
