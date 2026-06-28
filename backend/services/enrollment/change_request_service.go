package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrChangeRequestNotFound      = errors.New("enrollment change request not found")
	ErrChangeRequestNotAllowed    = errors.New("enrollment change request is not allowed")
	ErrChangeRequestInvalidStatus = errors.New("enrollment change request has invalid status")
	ErrChangeRequestInvalidData   = errors.New("enrollment change request data is invalid")
	ErrChangeRequestConflict      = errors.New("enrollment change request conflicts with newer enrollment data")
)

type CreateChangeRequestInput struct {
	Submission         SubmitRequest
	ParentNote         string
	CreatedByAccountID *int64
}

type ChangeRequestMessageInput struct {
	Body           string
	ActorAccountID int64
}

type ReviewChangeRequestInput struct {
	Note           string
	ActorAccountID int64
	ActorRole      string
}

type ChangeRequestDecisionApplier interface {
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error)
	SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*enrollmentModels.RequestChild, error)
}

type ChangeRequestAggregate struct {
	ChangeRequest *enrollmentModels.ChangeRequest
	Request       *enrollmentModels.Request
	Children      []*enrollmentModels.RequestChild
	Messages      []*enrollmentModels.ChangeRequestMessage
	Phase         *enrollmentModels.Phase
}

type ChangeRequestFilters struct {
	RequestID int64
	Status    string
	Limit     int
}

type ChangeRequestService interface {
	Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error)
	ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error)
	ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error)
	GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error)
	AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
	Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
}

type ChangeRequestServiceConfig struct {
	ChangeRequestRepo        enrollmentModels.ChangeRequestRepository
	MessageRepo              enrollmentModels.ChangeRequestMessageRepository
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestGuardianRepo      enrollmentModels.RequestGuardianRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	SchoolRepo               platformModels.SchoolRepository
	GuardianProfileRepo      userModels.GuardianProfileRepository
	GuardianPhoneRepo        userModels.GuardianPhoneNumberRepository
	DecisionService          ChangeRequestDecisionApplier
	Settings                 RequestSettingsResolver
	OutboxEnqueuer           OutboxEnqueuer
	FrontendURL              string
	ParentsURL               string
	DB                       *bun.DB
	Logger                   *slog.Logger
}

type changeRequestService struct {
	changeRequestRepo        enrollmentModels.ChangeRequestRepository
	messageRepo              enrollmentModels.ChangeRequestMessageRepository
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestGuardianRepo      enrollmentModels.RequestGuardianRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	phaseRepo                enrollmentModels.PhaseRepository
	schoolRepo               platformModels.SchoolRepository
	guardianProfileRepo      userModels.GuardianProfileRepository
	guardianPhoneRepo        userModels.GuardianPhoneNumberRepository
	decisionService          ChangeRequestDecisionApplier
	settings                 RequestSettingsResolver
	outboxEnqueuer           OutboxEnqueuer
	frontendURL              string
	parentsURL               string
	db                       *bun.DB
	logger                   *slog.Logger
}

func NewChangeRequestService(cfg ChangeRequestServiceConfig) ChangeRequestService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	parentsURL := strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
	if parentsURL == "" {
		parentsURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
	}
	return &changeRequestService{
		changeRequestRepo:        cfg.ChangeRequestRepo,
		messageRepo:              cfg.MessageRepo,
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestGuardianRepo:      cfg.RequestGuardianRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		careOfferingRepo:         cfg.CareOfferingRepo,
		formSchemaRepo:           cfg.FormSchemaRepo,
		phaseRepo:                cfg.PhaseRepo,
		schoolRepo:               cfg.SchoolRepo,
		guardianProfileRepo:      cfg.GuardianProfileRepo,
		guardianPhoneRepo:        cfg.GuardianPhoneRepo,
		decisionService:          cfg.DecisionService,
		settings:                 cfg.Settings,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		frontendURL:              strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		parentsURL:               parentsURL,
		db:                       cfg.DB,
		logger:                   logger,
	}
}

func (s *changeRequestService) Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	var created *enrollmentModels.ChangeRequest
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		lockedReq, err := s.requestRepo.FindByStatusTokenForUpdate(txCtx, strings.TrimSpace(token))
		if err != nil {
			return ErrRequestNotFound
		}
		children, err := s.requestChildRepo.ListByRequestIDForUpdate(txCtx, lockedReq.ID)
		if err != nil {
			return fmt.Errorf("change request: lock children: %w", err)
		}
		if err := s.ensureCanCreate(txCtx, lockedReq, children); err != nil {
			return err
		}
		if err := s.ensureNoOpenChangeRequest(txCtx, lockedReq.ID); err != nil {
			return err
		}
		prepared, _, _, err := s.prepareProposed(txCtx, lockedReq, children, input.Submission)
		if err != nil {
			return err
		}
		baseSnapshot, err := s.currentSnapshot(txCtx, lockedReq, children)
		if err != nil {
			return err
		}
		proposedSnapshot := submitSnapshot(prepared)
		note := strings.TrimSpace(input.ParentNote)
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		row := &enrollmentModels.ChangeRequest{
			RequestID:          lockedReq.ID,
			Status:             enrollmentModels.ChangeRequestStatusPendingReview,
			ParentNote:         notePtr,
			BaseSnapshot:       baseSnapshot,
			ProposedSnapshot:   proposedSnapshot,
			Diff:               snapshotDiff(baseSnapshot, proposedSnapshot),
			CreatedByAccountID: input.CreatedByAccountID,
		}
		if err := s.changeRequestRepo.Create(txCtx, row); err != nil {
			return err
		}
		if note != "" {
			if err := s.messageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
				ChangeRequestID: row.ID,
				AuthorType:      enrollmentModels.ChangeRequestMessageAuthorParent,
				AuthorAccountID: input.CreatedByAccountID,
				Body:            note,
			}); err != nil {
				return err
			}
		}
		created = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.enqueueChangeRequestSubmitted(ctx, tenantID, req, created)
	return s.loadAggregateForTenant(ctx, tenantID, created.ID, false)
}

func (s *changeRequestService) ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	var rows []*enrollmentModels.ChangeRequest
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		list, listErr := s.changeRequestRepo.ListByRequestID(txCtx, req.ID)
		rows = list
		return listErr
	})
	if err != nil {
		return nil, err
	}
	out := make([]*ChangeRequestAggregate, 0, len(rows))
	for _, row := range rows {
		agg, loadErr := s.loadAggregateForTenant(ctx, tenantID, row.ID, false)
		if loadErr != nil {
			return nil, loadErr
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *changeRequestService) ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrChangeRequestInvalidData)
	}
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		row, err := s.changeRequestRepo.FindByIDForUpdate(txCtx, changeRequestID)
		if err != nil || row == nil || row.RequestID != req.ID {
			return ErrChangeRequestNotFound
		}
		if row.Status != enrollmentModels.ChangeRequestStatusNeedsParentResponse {
			return ErrChangeRequestInvalidStatus
		}
		if err := s.messageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorParent,
			AuthorAccountID: nil,
			Body:            body,
		}); err != nil {
			return err
		}
		return s.changeRequestRepo.SetStatus(txCtx, row.ID, enrollmentModels.ChangeRequestStatusPendingReview)
	})
	if err != nil {
		return nil, err
	}
	s.enqueueParentReply(ctx, tenantID, req, changeRequestID)
	return s.loadAggregateForTenant(ctx, tenantID, changeRequestID, false)
}

func (s *changeRequestService) ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error) {
	rows, err := s.changeRequestRepo.ListAdmin(ctx, enrollmentModels.ChangeRequestListFilters{
		RequestID: filters.RequestID,
		Status:    filters.Status,
		Limit:     filters.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*ChangeRequestAggregate, 0, len(rows))
	for _, row := range rows {
		agg, err := s.loadAggregate(ctx, row.ID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *changeRequestService) GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error) {
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrChangeRequestInvalidData)
	}
	var req *enrollmentModels.Request
	err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *enrollmentModels.ChangeRequest) error {
		if row.Status != enrollmentModels.ChangeRequestStatusPendingReview {
			return ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.requestRepo.FindByID(txCtx, row.RequestID)
		if err != nil {
			return ErrRequestNotFound
		}
		req = loadedReq
		actorID := input.ActorAccountID
		if err := s.messageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            body,
		}); err != nil {
			return err
		}
		return s.changeRequestRepo.SetStatus(txCtx, row.ID, enrollmentModels.ChangeRequestStatusNeedsParentResponse)
	})
	if err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueAdminQuestion(ctx, req.GetTenantID(), req, changeRequestID)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: rejection note is required", ErrChangeRequestInvalidData)
	}
	var req *enrollmentModels.Request
	err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *enrollmentModels.ChangeRequest) error {
		if row.Status != enrollmentModels.ChangeRequestStatusPendingReview {
			return ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.requestRepo.FindByID(txCtx, row.RequestID)
		if err != nil {
			return ErrRequestNotFound
		}
		req = loadedReq
		now := time.Now()
		if err := s.changeRequestRepo.MarkReviewed(txCtx, row.ID, enrollmentModels.ChangeRequestStatusRejected, &note, input.ActorAccountID, now); err != nil {
			return err
		}
		actorID := input.ActorAccountID
		return s.messageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            note,
		})
	})
	if err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueReviewed(ctx, req.GetTenantID(), req, changeRequestID, platformModels.EmailKindEnrollmentChangeRequestRejected)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: approval note is required", ErrChangeRequestInvalidData)
	}
	var req *enrollmentModels.Request
	err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *enrollmentModels.ChangeRequest) error {
		if row.Status != enrollmentModels.ChangeRequestStatusPendingReview {
			return ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.requestRepo.FindByID(txCtx, row.RequestID)
		if err != nil {
			return ErrRequestNotFound
		}
		req = loadedReq
		if err := s.applyApprovedChange(txCtx, row, input); err != nil {
			return err
		}
		now := time.Now()
		if err := s.changeRequestRepo.MarkReviewed(txCtx, row.ID, enrollmentModels.ChangeRequestStatusApproved, &note, input.ActorAccountID, now); err != nil {
			return err
		}
		actorID := input.ActorAccountID
		return s.messageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            note,
		})
	})
	if err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueReviewed(ctx, req.GetTenantID(), req, changeRequestID, platformModels.EmailKindEnrollmentChangeRequestApproved)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) requestByToken(ctx context.Context, token string) (*enrollmentModels.Request, int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, 0, ErrRequestNotFound
	}
	var req *enrollmentModels.Request
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		loaded, err := s.requestRepo.FindByStatusToken(adminCtx, token)
		if err != nil {
			return ErrRequestNotFound
		}
		if loaded.StatusTokenExpires != nil && time.Now().After(*loaded.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		req = loaded
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return req, req.GetTenantID(), nil
}

func (s *changeRequestService) ensureCanCreate(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) error {
	rs := &requestService{settings: s.settings}
	if err := rs.ensureChangeRequestDraftAvailable(ctx, req, children); err != nil {
		return ErrChangeRequestNotAllowed
	}
	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil || !phase.IsActive {
		return ErrChangeRequestNotAllowed
	}
	if !IsEnrollmentWindowOpen(phase, time.Now()) {
		return ErrEnrollmentWindowClosed
	}
	return nil
}

func (s *changeRequestService) prepareProposed(
	ctx context.Context,
	req *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	incoming SubmitRequest,
) (SubmitRequest, [][]materializedOfferingSelection, *enrollmentModels.Phase, error) {
	editReq := incoming
	editReq.TenantID = req.GetTenantID()
	editReq.PhaseID = req.PhaseID
	editReq.GuardianEmail = req.GuardianEmail
	editReq.GuardianAccountID = req.GuardianAccountID
	if editReq.ConsentFlags == nil {
		editReq.ConsentFlags = map[string]any{}
	}
	if editReq.CustomData == nil {
		editReq.CustomData = map[string]any{}
	}
	if len(editReq.Children) != len(children) {
		return editReq, nil, nil, fmt.Errorf("%w: child count changes are not supported for change requests", ErrChangeRequestInvalidData)
	}
	if err := validateChangeRequestChildIdentity(children, editReq.Children); err != nil {
		return editReq, nil, nil, err
	}

	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil || !phase.IsActive {
		return editReq, nil, nil, ErrEnrollmentDisabled
	}
	openOfferings, err := s.careOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return editReq, nil, nil, fmt.Errorf("change request: load phase offerings: %w", err)
	}
	openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
	for _, offering := range openOfferings {
		openByID[offering.ID] = offering
	}
	materializedSelections, err := materializeAndValidateChildrenOfferingSelections(editReq.Children, openByID, phase.CareOfferingSelectionMode)
	if err != nil {
		return editReq, nil, nil, err
	}

	schema, err := s.schemaForRequest(ctx, req)
	if err != nil {
		return editReq, nil, nil, err
	}
	legalBlocks, err := s.legalBlocksForRequest(ctx, schema)
	if err != nil {
		return editReq, nil, nil, err
	}
	rs := &requestService{settings: s.settings, formSchemaRepo: s.formSchemaRepo, logger: s.logger}
	if err := normalizeAdditionalGuardians(&editReq); err != nil {
		return editReq, nil, nil, err
	}
	if err := rs.validateSubmission(ctx, editReq, legalBlocks); err != nil {
		return editReq, nil, nil, err
	}
	if err := s.validateAccountLinkedGuardianEdits(ctx, req, editReq); err != nil {
		return editReq, nil, nil, err
	}
	editReq.ConsentFlags = filterConsentFlags(editReq.ConsentFlags, legalBlocks)
	if err := rs.validateRequiredCustomFields(schema, editReq, openByID); err != nil {
		return editReq, nil, nil, err
	}
	if err := rs.validateAccompaniedCompanionNote(schema, editReq, openByID); err != nil {
		return editReq, nil, nil, err
	}
	if err := rs.validateConstrainedSchedules(schema, editReq, openByID); err != nil {
		return editReq, nil, nil, err
	}
	byKey := buildFieldsByKey(schema)
	rawGuardian := editReq.CustomData
	existingCustomData := existingChildCustomDataBySubmittedIdentity(children, editReq.Children)
	for i := range editReq.Children {
		childCtx := fieldVisibilityContext{
			guardianAnswers: rawGuardian,
			childAnswers:    editReq.Children[i].CustomData,
			gradeLevel:      editReq.Children[i].TargetGradeLevel,
			offeringNames:   selectedOfferingNames(editReq.Children[i], openByID),
			fieldsByKey:     byKey,
		}
		sanitizedChild := sanitizeVisibleAnswers(schema, true, editReq.Children[i].CustomData, childCtx)
		pruneChildScheduleAnswers(schema, sanitizedChild, relevantCareDaysForChild(editReq.Children[i], openByID))
		editReq.Children[i].CustomData = mergeEditableCustomData(existingCustomData[i], sanitizedChild, schema, true)
	}
	editReq.CustomData = mergeEditableCustomData(
		req.CustomData,
		sanitizeVisibleAnswers(schema, false, rawGuardian, fieldVisibilityContext{
			guardianAnswers: rawGuardian,
			fieldsByKey:     byKey,
		}),
		schema,
		false,
	)
	return editReq, materializedSelections, phase, nil
}

func (s *changeRequestService) schemaForRequest(ctx context.Context, req *enrollmentModels.Request) (*enrollmentModels.FormSchema, error) {
	if req.SchemaID == nil {
		return nil, nil
	}
	schema, err := s.formSchemaRepo.FindByID(ctx, *req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("change request: load schema: %w", err)
	}
	return schema, nil
}

func (s *changeRequestService) legalBlocksForRequest(ctx context.Context, schema *enrollmentModels.FormSchema) ([]LegalBlock, error) {
	texts, err := (&requestService{settings: s.settings}).LegalTexts(ctx)
	if err != nil {
		return nil, err
	}
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			texts.Blocks = blocks
		}
	}
	return texts.Blocks, nil
}

func (s *changeRequestService) currentSnapshot(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) (map[string]any, error) {
	guardians, err := s.requestGuardianRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("change request: list guardians: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	links, err := s.requestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("change request: list child offerings: %w", err)
	}
	linksByChild := make(map[int64][]*enrollmentModels.RequestChildOffering, len(children))
	for _, link := range links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}
	return persistedSnapshot(req, children, guardians, linksByChild), nil
}

func (s *changeRequestService) applyApprovedChange(ctx context.Context, row *enrollmentModels.ChangeRequest, input ReviewChangeRequestInput) error {
	req, err := s.requestRepo.FindByID(ctx, row.RequestID)
	if err != nil {
		return ErrRequestNotFound
	}
	children, err := s.requestChildRepo.ListByRequestIDForUpdate(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("change request approve: lock children: %w", err)
	}
	for _, child := range children {
		if child.Status == enrollmentModels.ChildStatusWithdrawn {
			return ErrChangeRequestNotAllowed
		}
	}
	current, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return err
	}
	if !jsonEqual(current, row.BaseSnapshot) {
		return ErrChangeRequestConflict
	}
	proposed, err := snapshotToSubmitRequest(row.ProposedSnapshot)
	if err != nil {
		return err
	}
	prepared, materializedSelections, _, err := s.prepareProposed(ctx, req, children, proposed)
	if err != nil {
		return err
	}
	var previousGuardians []*enrollmentModels.RequestGuardian
	if s.requestGuardianRepo != nil {
		previousGuardians, err = s.requestGuardianRepo.ListByRequestID(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("change request approve: list previous guardians: %w", err)
		}
	}

	req.GuardianFirstName = strings.TrimSpace(prepared.GuardianFirstName)
	req.GuardianLastName = strings.TrimSpace(prepared.GuardianLastName)
	req.GuardianEmail = strings.ToLower(strings.TrimSpace(prepared.GuardianEmail))
	req.GuardianPhone = prepared.GuardianPhone
	req.ConsentFlags = prepared.ConsentFlags
	req.CustomData = prepared.CustomData
	if err := s.requestRepo.UpdateGuardianDataWithEmail(ctx, req); err != nil {
		return err
	}
	if s.requestGuardianRepo != nil {
		if err := s.requestGuardianRepo.DeleteByRequestID(ctx, req.ID); err != nil {
			return err
		}
		for i, guardian := range prepared.AdditionalGuardians {
			if err := s.requestGuardianRepo.Create(ctx, &enrollmentModels.RequestGuardian{
				RequestID: req.ID,
				FirstName: guardian.FirstName,
				LastName:  guardian.LastName,
				Email:     guardian.Email,
				Phone:     guardian.Phone,
				SortOrder: i,
			}); err != nil {
				return fmt.Errorf("change request approve: create guardian %d: %w", i, err)
			}
		}
	}

	for i, existing := range children {
		next := prepared.Children[i]
		existing.FirstName = strings.TrimSpace(next.FirstName)
		existing.LastName = strings.TrimSpace(next.LastName)
		existing.DateOfBirth = next.DateOfBirth
		existing.TargetGradeLevel = next.TargetGradeLevel
		existing.CustomData = next.CustomData
		existing.SortOrder = i
		if err := s.requestChildRepo.UpdateData(ctx, existing); err != nil {
			return err
		}
		selections := materializedSelections[i]
		if existing.Status == enrollmentModels.ChildStatusApproved && existing.CreatedStudentID != nil && s.decisionService != nil {
			offeringInput := UpdateChildOfferingsInput{
				RequestID:      req.ID,
				ChildID:        existing.ID,
				Reason:         input.Note,
				ActorAccountID: input.ActorAccountID,
				ActorRole:      input.ActorRole,
			}
			for _, selection := range selections {
				offeringInput.Offerings = append(offeringInput.Offerings, OfferingAdjustmentSelection{
					OfferingID:   selection.OfferingID,
					SelectedDays: selection.SelectedDays,
				})
			}
			if _, err := s.decisionService.UpdateChildOfferings(ctx, offeringInput); err != nil {
				return err
			}
			if _, err := s.decisionService.SyncApprovedChildData(ctx, SyncApprovedChildDataInput{
				RequestID:                req.ID,
				ChildID:                  existing.ID,
				ActorAccountID:           input.ActorAccountID,
				ReplaceTargetedData:      true,
				PreviousSnapshot:         row.BaseSnapshot,
				PreviousRequestGuardians: previousGuardians,
			}); err != nil {
				return err
			}
			continue
		}
		replacement := make([]*enrollmentModels.RequestChildOffering, 0, len(selections))
		for _, selection := range selections {
			replacement = append(replacement, &enrollmentModels.RequestChildOffering{
				RequestChildID:        existing.ID,
				CareOfferingID:        selection.OfferingID,
				SelectedDays:          selection.SelectedDays,
				ManualSelectedDays:    selection.ManualSelectedDays,
				AutomaticSelectedDays: selection.AutomaticSelectedDays,
			})
		}
		if err := s.requestChildOfferingRepo.ReplaceForRequestChild(ctx, existing.ID, replacement); err != nil {
			return err
		}
		if existing.Status == enrollmentModels.ChildStatusRejected && childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, existing.ID) {
			if err := s.requestChildRepo.UpdateStatus(ctx, existing.ID, enrollmentModels.ChildStatusUnderReview, nil, input.ActorAccountID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *changeRequestService) ensureNoOpenChangeRequest(ctx context.Context, requestID int64) error {
	rows, err := s.changeRequestRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == enrollmentModels.ChangeRequestStatusPendingReview ||
			row.Status == enrollmentModels.ChangeRequestStatusNeedsParentResponse {
			return fmt.Errorf("%w: an open change request already exists", ErrChangeRequestNotAllowed)
		}
	}
	return nil
}

func validateChangeRequestChildIdentity(existing []*enrollmentModels.RequestChild, incoming []SubmitChild) error {
	if len(existing) != len(incoming) {
		return fmt.Errorf("%w: child count changes are not supported for change requests", ErrChangeRequestInvalidData)
	}
	for i, child := range existing {
		if incoming[i].ID <= 0 {
			return fmt.Errorf("%w: child %d id is required", ErrChangeRequestInvalidData, i)
		}
		if incoming[i].ID != child.ID {
			return fmt.Errorf("%w: child order or identity changes are not supported for change requests", ErrChangeRequestInvalidData)
		}
	}
	return nil
}

func childSnapshotChanged(base, proposed map[string]any, childID int64) bool {
	baseChild := snapshotChildByID(base, childID)
	proposedChild := snapshotChildByID(proposed, childID)
	if baseChild == nil || proposedChild == nil {
		return false
	}
	baseComparable := copySnapshotMapWithoutStatus(baseChild)
	proposedComparable := copySnapshotMapWithoutStatus(proposedChild)
	return !jsonEqual(baseComparable, proposedComparable)
}

func snapshotChildByID(snapshot map[string]any, childID int64) map[string]any {
	for _, raw := range sliceFromAny(snapshot["children"]) {
		row := mapFromAny(raw)
		if int64FromAny(row["id"]) == childID {
			return row
		}
	}
	return nil
}

func copySnapshotMapWithoutStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		if key == "status" {
			continue
		}
		out[key] = value
	}
	return out
}

func (s *changeRequestService) validateAccountLinkedGuardianEdits(ctx context.Context, req *enrollmentModels.Request, editReq SubmitRequest) error {
	if s.guardianProfileRepo == nil {
		return nil
	}
	profile, err := s.primaryGuardianProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("change request: load primary guardian profile for account guardrail: %w", err)
	}
	if profile != nil && profile.HasPortalAccount() {
		if !sameTrimmedString(editReq.GuardianFirstName, req.GuardianFirstName) ||
			!sameTrimmedString(editReq.GuardianLastName, req.GuardianLastName) ||
			!sameOptionalString(editReq.GuardianPhone, req.GuardianPhone) {
			return fmt.Errorf("%w: account-linked guardian profile details must be changed in the parent portal", ErrChangeRequestInvalidData)
		}
	}
	if len(editReq.AdditionalGuardians) == 0 || s.requestGuardianRepo == nil {
		return nil
	}
	existing, err := s.requestGuardianRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("change request: list guardians for account guardrail: %w", err)
	}
	for i, guardian := range editReq.AdditionalGuardians {
		email := optionalLowerEmail(guardian.Email)
		if email == "" {
			continue
		}
		profile, err := s.guardianProfileRepo.FindByEmail(ctx, email)
		if err != nil {
			if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
				continue
			}
			return fmt.Errorf("change request: load co-guardian profile for account guardrail: %w", err)
		}
		if profile == nil || !profile.HasPortalAccount() {
			continue
		}
		if old := matchingRequestGuardian(existing, profile.ID, email); old != nil {
			if !sameTrimmedString(guardian.FirstName, old.FirstName) ||
				!sameTrimmedString(guardian.LastName, old.LastName) ||
				!sameOptionalString(guardian.Phone, old.Phone) {
				return fmt.Errorf("%w: account-linked co-guardian %d details must be changed in the parent portal", ErrChangeRequestInvalidData, i)
			}
			continue
		}
		if !sameTrimmedString(guardian.FirstName, profile.FirstName) ||
			!sameTrimmedString(guardian.LastName, profile.LastName) ||
			!s.submittedPhoneMatchesProfile(ctx, profile.ID, guardian.Phone) {
			return fmt.Errorf("%w: account-linked co-guardian %d details must match the parent portal profile", ErrChangeRequestInvalidData, i)
		}
	}
	return nil
}

func (s *changeRequestService) primaryGuardianProfile(ctx context.Context, req *enrollmentModels.Request) (*userModels.GuardianProfile, error) {
	if s.guardianProfileRepo == nil || req == nil {
		return nil, nil
	}
	if req.GuardianAccountID != nil && *req.GuardianAccountID > 0 {
		profile, err := s.guardianProfileRepo.FindByAccountID(ctx, *req.GuardianAccountID)
		if err == nil && profile != nil {
			return profile, nil
		}
		if err != nil && !errors.Is(err, userModels.ErrGuardianProfileNotFound) {
			return nil, err
		}
	}
	email := strings.TrimSpace(strings.ToLower(req.GuardianEmail))
	if email == "" {
		return nil, nil
	}
	profile, err := s.guardianProfileRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return profile, nil
}

func matchingRequestGuardian(rows []*enrollmentModels.RequestGuardian, profileID int64, email string) *enrollmentModels.RequestGuardian {
	email = strings.TrimSpace(strings.ToLower(email))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.GuardianProfileID != nil && *row.GuardianProfileID == profileID {
			return row
		}
		if email != "" && optionalLowerEmail(row.Email) == email {
			return row
		}
	}
	return nil
}

func (s *changeRequestService) submittedPhoneMatchesProfile(ctx context.Context, profileID int64, submitted *string) bool {
	phone := trimmedOptionalString(submitted)
	if phone == "" {
		return true
	}
	if s.guardianPhoneRepo == nil {
		return false
	}
	rows, err := s.guardianPhoneRepo.FindByGuardianID(ctx, profileID)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.PhoneNumber) == phone {
			return true
		}
	}
	return false
}

func sameTrimmedString(left, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func sameOptionalString(left, right *string) bool {
	return trimmedOptionalString(left) == trimmedOptionalString(right)
}

func trimmedOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func optionalLowerEmail(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(*value))
}

func (s *changeRequestService) withLockedChangeRequest(ctx context.Context, id int64, fn func(context.Context, *enrollmentModels.ChangeRequest) error) error {
	row, err := s.changeRequestRepo.FindByIDForUpdate(ctx, id)
	if err != nil || row == nil {
		return ErrChangeRequestNotFound
	}
	return fn(ctx, row)
}

func (s *changeRequestService) loadAggregate(ctx context.Context, id int64, includeInternal bool) (*ChangeRequestAggregate, error) {
	row, err := s.changeRequestRepo.FindByID(ctx, id)
	if err != nil || row == nil {
		return nil, ErrChangeRequestNotFound
	}
	return s.aggregateFromRow(ctx, row, includeInternal)
}

func (s *changeRequestService) loadAggregateForTenant(ctx context.Context, tenantID int64, id int64, includeInternal bool) (*ChangeRequestAggregate, error) {
	var agg *ChangeRequestAggregate
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		loaded, err := s.loadAggregate(txCtx, id, includeInternal)
		agg = loaded
		return err
	})
	return agg, err
}

func (s *changeRequestService) aggregateFromRow(ctx context.Context, row *enrollmentModels.ChangeRequest, includeInternal bool) (*ChangeRequestAggregate, error) {
	req, err := s.requestRepo.FindByID(ctx, row.RequestID)
	if err != nil {
		return nil, ErrRequestNotFound
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("change request: list aggregate children: %w", err)
	}
	messages, err := s.messageRepo.ListByChangeRequestID(ctx, row.ID, includeInternal)
	if err != nil {
		return nil, err
	}
	var phase *enrollmentModels.Phase
	if s.phaseRepo != nil {
		phase, _ = s.phaseRepo.FindByID(ctx, req.PhaseID)
	}
	return &ChangeRequestAggregate{
		ChangeRequest: row,
		Request:       req,
		Children:      children,
		Messages:      messages,
		Phase:         phase,
	}, nil
}

func submitSnapshot(req SubmitRequest) map[string]any {
	children := make([]any, 0, len(req.Children))
	for _, child := range req.Children {
		offeringIDs := make([]any, 0, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			offeringIDs = append(offeringIDs, strconv.FormatInt(id, 10))
		}
		offeringDays := make([]any, 0, len(child.OfferingDays))
		for _, row := range child.OfferingDays {
			offeringDays = append(offeringDays, map[string]any{
				"offering_id":   strconv.FormatInt(row.OfferingID, 10),
				"selected_days": copyDays(row.SelectedDays),
			})
		}
		row := map[string]any{
			"first_name":         child.FirstName,
			"last_name":          child.LastName,
			"date_of_birth":      child.DateOfBirth.String(),
			"target_grade_level": child.TargetGradeLevel,
			"custom_data":        child.CustomData,
			"offering_ids":       offeringIDs,
			"offering_days":      offeringDays,
		}
		if child.ID > 0 {
			row["id"] = strconv.FormatInt(child.ID, 10)
		}
		children = append(children, row)
	}
	guardians := make([]any, 0, len(req.AdditionalGuardians))
	for _, guardian := range req.AdditionalGuardians {
		guardians = append(guardians, map[string]any{
			"first_name": guardian.FirstName,
			"last_name":  guardian.LastName,
			"email":      ptrStringValue(guardian.Email),
			"phone":      ptrStringValue(guardian.Phone),
		})
	}
	return map[string]any{
		"phase_id":             strconv.FormatInt(req.PhaseID, 10),
		"guardian_first_name":  req.GuardianFirstName,
		"guardian_last_name":   req.GuardianLastName,
		"guardian_email":       req.GuardianEmail,
		"guardian_phone":       ptrStringValue(req.GuardianPhone),
		"consent_flags":        req.ConsentFlags,
		"custom_data":          req.CustomData,
		"additional_guardians": guardians,
		"children":             children,
	}
}

func persistedSnapshot(
	req *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	guardians []*enrollmentModels.RequestGuardian,
	linksByChild map[int64][]*enrollmentModels.RequestChildOffering,
) map[string]any {
	submit := SubmitRequest{
		TenantID:          req.GetTenantID(),
		PhaseID:           req.PhaseID,
		GuardianFirstName: req.GuardianFirstName,
		GuardianLastName:  req.GuardianLastName,
		GuardianEmail:     req.GuardianEmail,
		GuardianPhone:     req.GuardianPhone,
		ConsentFlags:      req.ConsentFlags,
		CustomData:        req.CustomData,
	}
	for _, guardian := range guardians {
		submit.AdditionalGuardians = append(submit.AdditionalGuardians, SubmitGuardian{
			FirstName: guardian.FirstName,
			LastName:  guardian.LastName,
			Email:     guardian.Email,
			Phone:     guardian.Phone,
		})
	}
	for _, child := range children {
		next := SubmitChild{
			FirstName:        child.FirstName,
			LastName:         child.LastName,
			DateOfBirth:      child.DateOfBirth,
			TargetGradeLevel: child.TargetGradeLevel,
			CustomData:       child.CustomData,
		}
		for _, link := range linksByChild[child.ID] {
			next.OfferingIDs = append(next.OfferingIDs, link.CareOfferingID)
			if len(link.SelectedDays) > 0 {
				next.OfferingDays = append(next.OfferingDays, SubmitOfferingDays{
					OfferingID:   link.CareOfferingID,
					SelectedDays: copyDays(link.SelectedDays),
				})
			}
		}
		submit.Children = append(submit.Children, next)
	}
	snapshot := submitSnapshot(submit)
	rows := snapshot["children"].([]any)
	for i, child := range children {
		if i >= len(rows) {
			continue
		}
		if row, ok := rows[i].(map[string]any); ok {
			row["id"] = strconv.FormatInt(child.ID, 10)
			row["status"] = child.Status
		}
	}
	return snapshot
}

func snapshotDiff(base, proposed map[string]any) map[string]any {
	changed := make([]string, 0)
	keys := make(map[string]bool)
	for key := range base {
		keys[key] = true
	}
	for key := range proposed {
		keys[key] = true
	}
	for key := range keys {
		if !jsonEqual(base[key], proposed[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return map[string]any{
		"changed": changed,
	}
}

func jsonEqual(left, right any) bool {
	normalize := func(v any) any {
		raw, err := json.Marshal(v)
		if err != nil {
			return v
		}
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return v
		}
		return out
	}
	return reflect.DeepEqual(normalize(left), normalize(right))
}

func snapshotToSubmitRequest(snapshot map[string]any) (SubmitRequest, error) {
	var out SubmitRequest
	out.PhaseID = int64FromAny(snapshot["phase_id"])
	out.GuardianFirstName = stringFromAny(snapshot["guardian_first_name"])
	out.GuardianLastName = stringFromAny(snapshot["guardian_last_name"])
	out.GuardianEmail = stringFromAny(snapshot["guardian_email"])
	out.GuardianPhone = optionalStringFromAny(snapshot["guardian_phone"])
	out.ConsentFlags = mapFromAny(snapshot["consent_flags"])
	out.CustomData = mapFromAny(snapshot["custom_data"])
	for _, raw := range sliceFromAny(snapshot["additional_guardians"]) {
		row := mapFromAny(raw)
		out.AdditionalGuardians = append(out.AdditionalGuardians, SubmitGuardian{
			FirstName: stringFromAny(row["first_name"]),
			LastName:  stringFromAny(row["last_name"]),
			Email:     optionalStringFromAny(row["email"]),
			Phone:     optionalStringFromAny(row["phone"]),
		})
	}
	for i, raw := range sliceFromAny(snapshot["children"]) {
		row := mapFromAny(raw)
		dob, err := timezone.ParseDate(stringFromAny(row["date_of_birth"]))
		if err != nil {
			return out, fmt.Errorf("%w: child %d date_of_birth", ErrChangeRequestInvalidData, i)
		}
		child := SubmitChild{
			ID:               int64FromAny(row["id"]),
			FirstName:        stringFromAny(row["first_name"]),
			LastName:         stringFromAny(row["last_name"]),
			DateOfBirth:      dob,
			TargetGradeLevel: int16PtrFromAny(row["target_grade_level"]),
			CustomData:       mapFromAny(row["custom_data"]),
		}
		for _, rawID := range sliceFromAny(row["offering_ids"]) {
			if id := int64FromAny(rawID); id > 0 {
				child.OfferingIDs = append(child.OfferingIDs, id)
			}
		}
		for _, rawDay := range sliceFromAny(row["offering_days"]) {
			day := mapFromAny(rawDay)
			id := int64FromAny(day["offering_id"])
			if id <= 0 {
				continue
			}
			child.OfferingDays = append(child.OfferingDays, SubmitOfferingDays{
				OfferingID:   id,
				SelectedDays: stringSliceFromAny(day["selected_days"]),
			})
		}
		out.Children = append(out.Children, child)
	}
	return out, nil
}

func ptrStringValue(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func optionalStringFromAny(v any) *string {
	s := strings.TrimSpace(stringFromAny(v))
	if s == "" {
		return nil
	}
	return &s
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n
	default:
		return 0
	}
}

func int16PtrFromAny(v any) *int16 {
	n := int64FromAny(v)
	if n == 0 {
		return nil
	}
	out := int16(n)
	return &out
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

func sliceFromAny(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	if stringsSlice, ok := v.([]string); ok {
		out := make([]any, 0, len(stringsSlice))
		for _, value := range stringsSlice {
			out = append(out, value)
		}
		return out
	}
	return nil
}

func stringSliceFromAny(v any) []string {
	out := make([]string, 0)
	for _, raw := range sliceFromAny(v) {
		if s := stringFromAny(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (s *changeRequestService) emailNotificationsEnabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled)
	return err == nil && enabled
}

func (s *changeRequestService) enqueueChangeRequestSubmitted(ctx context.Context, tenantID int64, req *enrollmentModels.Request, cr *enrollmentModels.ChangeRequest) {
	if s.outboxEnqueuer == nil {
		return
	}
	kind := platformModels.EmailKindEnrollmentChangeRequestSubmitted
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		for _, admin := range (&requestService{settings: s.settings}).resolveAdminEmails(txCtx) {
			payload := s.emailPayload(txCtx, req, cr.ID, admin)
			payload[EnrollmentPayloadAdminURL] = s.adminURL(cr.ID)
			if enqueueErr := s.outboxEnqueuer.Enqueue(txCtx, OutboxEnqueueRequest{
				Kind:              kind,
				Payload:           payload,
				RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
				RelatedEntityID:   req.ID,
			}); enqueueErr != nil {
				s.logChangeRequestNotificationFailure(enqueueErr, tenantID, req.ID, cr.ID, kind, admin, "enqueue")
			}
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, cr.ID, kind, "", "tenant_tx")
	}
}

func (s *changeRequestService) enqueueParentReply(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64) {
	if s.outboxEnqueuer == nil {
		return
	}
	kind := platformModels.EmailKindEnrollmentChangeRequestParentReply
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		for _, admin := range (&requestService{settings: s.settings}).resolveAdminEmails(txCtx) {
			payload := s.emailPayload(txCtx, req, changeRequestID, admin)
			payload[EnrollmentPayloadAdminURL] = s.adminURL(changeRequestID)
			if enqueueErr := s.outboxEnqueuer.Enqueue(txCtx, OutboxEnqueueRequest{
				Kind:              kind,
				Payload:           payload,
				RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
				RelatedEntityID:   req.ID,
			}); enqueueErr != nil {
				s.logChangeRequestNotificationFailure(enqueueErr, tenantID, req.ID, changeRequestID, kind, admin, "enqueue")
			}
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, "", "tenant_tx")
	}
}

func (s *changeRequestService) enqueueAdminQuestion(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64) {
	s.enqueueParentNotification(ctx, tenantID, req, changeRequestID, platformModels.EmailKindEnrollmentChangeRequestQuestion)
}

func (s *changeRequestService) enqueueReviewed(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	s.enqueueParentNotification(ctx, tenantID, req, changeRequestID, kind)
}

func (s *changeRequestService) enqueueParentNotification(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	if s.outboxEnqueuer == nil {
		return
	}
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		payload := s.emailPayload(txCtx, req, changeRequestID, req.GuardianEmail)
		if enqueueErr := s.outboxEnqueuer.Enqueue(txCtx, OutboxEnqueueRequest{
			Kind:              kind,
			Payload:           payload,
			RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
			RelatedEntityID:   req.ID,
		}); enqueueErr != nil {
			s.logChangeRequestNotificationFailure(enqueueErr, tenantID, req.ID, changeRequestID, kind, req.GuardianEmail, "enqueue")
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, req.GuardianEmail, "tenant_tx")
	}
}

func (s *changeRequestService) logChangeRequestNotificationFailure(err error, tenantID, requestID, changeRequestID int64, kind, recipient, stage string) {
	if err == nil || s.logger == nil {
		return
	}
	s.logger.Warn("change request notification enqueue failed",
		slog.String("stage", stage),
		slog.Int64("tenant_id", tenantID),
		slog.Int64("request_id", requestID),
		slog.Int64("change_request_id", changeRequestID),
		slog.String("kind", kind),
		slog.String("recipient", recipient),
		slog.String("error", err.Error()),
	)
}

func (s *changeRequestService) emailPayload(ctx context.Context, req *enrollmentModels.Request, changeRequestID int64, recipient string) map[string]any {
	schoolName, logoURL := emailBrandForSchool(ctx, s.schoolRepo, req.GetTenantID(), s.parentsURL)
	return map[string]any{
		EnrollmentPayloadGuardianFirstName: req.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  req.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     req.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         fmt.Sprintf("%s/enroll/status/%s", s.parentsURL, req.StatusToken),
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       motoLogoURL(s.parentsURL),
		EnrollmentPayloadRecipientEmail:    recipient,
		"change_request_id":                strconv.FormatInt(changeRequestID, 10),
	}
}

func (s *changeRequestService) adminURL(changeRequestID int64) string {
	if s.frontendURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/admin/enrollments/change-requests/%d", s.frontendURL, changeRequestID)
}
