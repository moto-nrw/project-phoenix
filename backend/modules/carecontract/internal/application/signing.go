package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The signature of a contract (#4679, ADR 0088 section 3): the one-time
// mail link, the signature in the parents portal, the paper contract and
// the signature on the contract's detail.

type (
	SignatureStore    = ports.SignatureStore
	StoredDocument    = ports.StoredDocument
	StoredSignature   = ports.StoredSignature
	NewSignature      = ports.NewSignature
	SigningLinkRecord = ports.SigningLinkRecord
	SentState         = ports.SentState
)

// ContractStore is the store of the contracts with their signatures.
type ContractStore interface {
	Store
	SignatureStore
}

// SignaturePorts are the root-bound ports of the signature.
type SignaturePorts struct {
	Ledger  carecontract.SignatureLedger
	Links   carecontract.SigningLinks
	Sender  carecontract.TraegerSender
	Post    carecontract.Post
	Runtime carecontract.Runtime
}

// Signatures serves carecontract.Signing, carecontract.InboxSigning and
// carecontract.PaperSignatures.
type Signatures struct {
	store ContractStore
	ports SignaturePorts
	clock Clock
	// contracts freezes the PDF of a draft signed on paper; nil where only
	// the parents sign.
	contracts *Contracts
}

var (
	_ carecontract.Signing         = (*Signatures)(nil)
	_ carecontract.InboxSigning    = (*Signatures)(nil)
	_ carecontract.PaperSignatures = (*Signatures)(nil)
)

// NewSignatures builds the signature over the root-bound ports.
func NewSignatures(store ContractStore, ports SignaturePorts, clock Clock, contracts *Contracts) *Signatures {
	if store == nil || ports.Ledger == nil || ports.Links.Tokens == nil || ports.Links.Fingerprint == nil || ports.Sender == nil || ports.Post == nil || ports.Runtime == nil {
		panic("care contract: the store, the ledger, the tokens, the sender, the post and the runtime are required")
	}
	return &Signatures{store: store, ports: ports, clock: clock, contracts: contracts}
}

// parseLink splits "<Träger>.<token>"; a malformed link is an invalid one.
func parseLink(link string) (int64, string, error) {
	organization, token, found := strings.Cut(strings.TrimSpace(link), ".")
	id, err := strconv.ParseInt(organization, 10, 64)
	if !found || err != nil || id <= 0 || token == "" || len(token) > 128 {
		return 0, "", carecontract.ErrSigningLinkInvalid
	}
	return id, token, nil
}

// linkTarget is a contract that an open link names, with its frozen PDF.
type linkTarget struct {
	link     SigningLinkRecord
	record   ContractRecord
	document StoredDocument
}

// openLink resolves an open link in the Träger transaction: it must be
// unused and unexpired, its contract must wait for a signature, and the
// PDF it was sent with must still be the contract's PDF. Only failed link
// validation answers ErrSigningLinkInvalid; operational errors stay retryable.
func (s *Signatures) openLink(ctx context.Context, organizationID int64, token string, now time.Time) (linkTarget, error) {
	link, err := s.store.SigningLink(ctx, organizationID, s.ports.Links.Tokens.Fingerprint(token))
	if err != nil {
		return linkTarget{}, err
	}
	if link.UsedAt != nil || !link.ExpiresAt.After(now) {
		return linkTarget{}, carecontract.ErrSigningLinkInvalid
	}
	record, err := s.store.Contract(ctx, organizationID, link.ContractID, true)
	if errors.Is(err, carecontract.ErrContractNotFound) {
		return linkTarget{}, carecontract.ErrSigningLinkInvalid
	}
	if err != nil {
		return linkTarget{}, err
	}
	document, err := s.store.CurrentDocument(ctx, organizationID, link.ContractID, record.Version.ID)
	if errors.Is(err, carecontract.ErrContractNotSignable) {
		return linkTarget{}, carecontract.ErrSigningLinkInvalid
	}
	if err != nil {
		return linkTarget{}, err
	}
	if document.ID != link.DocumentID || record.Summary.Status != carecontract.StatusSent {
		return linkTarget{}, carecontract.ErrSigningLinkInvalid
	}
	return linkTarget{link: link, record: record, document: document}, nil
}

// LinkSigning shows the contract of an open link. It records the read and
// leaves the link open.
func (s *Signatures) LinkSigning(ctx context.Context, link string) (carecontract.SigningView, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	var view carecontract.SigningView
	err = s.withinLink(ctx, organizationID, func(txCtx context.Context) error {
		target, err := s.openLink(txCtx, organizationID, token, s.clock.now())
		if err != nil {
			return err
		}
		if view, err = s.view(txCtx, organizationID, target.record, target.document); err != nil {
			return err
		}
		return s.event(txCtx, organizationID, target.record, 0, ports.ActionRead, map[string]any{"via": carecontract.ChannelMailLink})
	})
	return view, err
}

// LinkSigningDocument answers the frozen PDF of an open link.
func (s *Signatures) LinkSigningDocument(ctx context.Context, link string) (carecontract.File, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.File{}, err
	}
	var file carecontract.File
	err = s.withinLink(ctx, organizationID, func(txCtx context.Context) error {
		target, err := s.openLink(txCtx, organizationID, token, s.clock.now())
		if err != nil {
			return err
		}
		file = target.document.File
		return s.event(txCtx, organizationID, target.record, 0, ports.ActionRead,
			map[string]any{"via": carecontract.ChannelMailLink, "view": "document"})
	})
	return file, err
}

// SignByLink signs the contract of an open link and closes the link.
func (s *Signatures) SignByLink(ctx context.Context, link string, input carecontract.SignatureInput) (carecontract.SigningView, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	var view carecontract.SigningView
	err = s.withinLink(ctx, organizationID, func(txCtx context.Context) error {
		now := s.clock.now()
		target, err := s.openLink(txCtx, organizationID, token, now)
		if err != nil {
			return err
		}
		if err := s.store.UseSigningLink(txCtx, organizationID, target.link.ID, now); err != nil {
			return err
		}
		linkID := target.link.ID
		sent := target.link.Sent
		view, err = s.sign(txCtx, organizationID, target.record, target.document, input, signer{
			channel: carecontract.ChannelMailLink, linkID: &linkID, sent: &sent,
		}, now)
		return err
	})
	return view, err
}

// withinLink opens the Träger transaction without reclassifying operational failures.
func (s *Signatures) withinLink(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	if err := s.ports.Runtime.WithinTraeger(ctx, organizationID, fn); err != nil {
		return fmt.Errorf("care contract: signing link transaction: %w", err)
	}
	return nil
}

// InboxSigningView shows a contract addressed to the reader, for one of
// the children the reader may sign for.
func (s *Signatures) InboxSigningView(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.SigningView, error) {
	record, document, err := s.inboxContract(ctx, organizationID, contractID, reader, false)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	view, err := s.view(ctx, organizationID, record, document)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	stored, err := s.store.Signature(ctx, organizationID, record.Summary.ID, record.Version.ID)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	if stored != nil {
		view.Signature = &carecontract.SigningSignature{
			Method: stored.Method, SignerName: stored.SignerName, SignedAt: stored.SignedAt,
			EarlyStartRequested: stored.EarlyStartRequested,
		}
		if stored.Method != carecontract.SignaturePaper {
			ends := withdrawalEnd(stored.SignedAt)
			view.Signature.WithdrawalEndsOn = &ends
		}
	}
	return view, s.event(ctx, organizationID, record, reader.AccountID, ports.ActionRead, map[string]any{"via": carecontract.ChannelParentAccount})
}

// SignInInbox signs a contract addressed to the reader.
func (s *Signatures) SignInInbox(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader, input carecontract.SignatureInput) (carecontract.SigningView, error) {
	record, document, err := s.inboxContract(ctx, organizationID, contractID, reader, true)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	account := reader.AccountID
	// The parents sign what the newest send gave them, the PDF and the
	// version of its link.
	link, found, err := s.store.LatestSigningLink(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	var sent *SentState
	if found {
		sent = &link.Sent
	}
	return s.sign(ctx, organizationID, record, document, input,
		signer{channel: carecontract.ChannelParentAccount, accountID: &account, sent: sent}, s.now())
}

// inboxContract reads a contract the reader may see and sign: sent to the
// reader's address, for a child the reader acts for. Any other contract is
// not in the inbox.
func (s *Signatures) inboxContract(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader, forUpdate bool) (ContractRecord, StoredDocument, error) {
	email, err := inboxAddress(organizationID, reader)
	if err != nil || contractID <= 0 {
		return ContractRecord{}, StoredDocument{}, carecontract.ErrInboxDocumentNotFound
	}
	reader.Email = email
	addressed, err := s.store.InboxContractAddressed(ctx, organizationID, contractID, reader)
	if err != nil {
		return ContractRecord{}, StoredDocument{}, err
	}
	if !addressed {
		return ContractRecord{}, StoredDocument{}, carecontract.ErrInboxDocumentNotFound
	}
	record, err := s.store.Contract(ctx, organizationID, contractID, forUpdate)
	if errors.Is(err, carecontract.ErrContractNotFound) {
		return ContractRecord{}, StoredDocument{}, carecontract.ErrInboxDocumentNotFound
	}
	if err != nil {
		return ContractRecord{}, StoredDocument{}, err
	}
	document, err := s.store.CurrentDocument(ctx, organizationID, contractID, record.Version.ID)
	if errors.Is(err, carecontract.ErrContractNotSignable) {
		return ContractRecord{}, StoredDocument{}, carecontract.ErrInboxDocumentNotFound
	}
	if err != nil {
		return ContractRecord{}, StoredDocument{}, err
	}
	return record, document, nil
}

// unchangedChecksum answers the checksum of the version the parents sign,
// once it is clear that neither it nor the PDF changed after the send.
func (s *Signatures) unchangedChecksum(record ContractRecord, document StoredDocument, sent *SentState) (string, error) {
	checksum, err := versionChecksum(s.ports.Links.Fingerprint, record.Version)
	if err != nil {
		return "", err
	}
	return checksum, unchangedSinceSent(sent, document, checksum)
}

// unchangedSinceSent refuses a signature of a contract whose frozen PDF or
// version no longer matches what was sent: the parents sign only what they
// were given. A contract without a send to compare with is not signable.
func unchangedSinceSent(sent *SentState, document StoredDocument, versionChecksum string) error {
	if sent == nil {
		return fmt.Errorf("%w: no send to compare the contract with", carecontract.ErrContractNotSignable)
	}
	if sent.DocumentSHA256 != document.SHA256 || sent.VersionSHA256 != versionChecksum {
		return fmt.Errorf("%w: the contract changed after it was sent", carecontract.ErrContractNotSignable)
	}
	return nil
}

// signer is who signs, how the signer was identified, and what was sent to
// the signer.
type signer struct {
	channel   string
	accountID *int64
	linkID    *int64
	sent      *SentState
}

// sign records a drawn or typed signature of the contract's frozen PDF,
// concludes the contract and enqueues the confirmation with the PDF and
// the withdrawal notice, all in the caller's Träger transaction.
func (s *Signatures) sign(ctx context.Context, organizationID int64, record ContractRecord, document StoredDocument,
	input carecontract.SignatureInput, who signer, now time.Time) (carecontract.SigningView, error) {
	if record.Summary.Status != carecontract.StatusSent || document.VersionID != record.Version.ID {
		return carecontract.SigningView{}, carecontract.ErrContractNotSignable
	}
	checksum, err := s.unchangedChecksum(record, document, who.sent)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	if !input.NoticeAcknowledged {
		return carecontract.SigningView{}, carecontract.ErrWithdrawalNoticeRequired
	}
	party, _ := recipient(record.Parties)
	signerName := fullName(party.FirstName, party.LastName)
	if input.Method == carecontract.SignatureTyped && strings.TrimSpace(input.TypedName) != "" {
		signerName = strings.TrimSpace(input.TypedName)
	}
	if signerName == "" {
		return carecontract.SigningView{}, carecontract.ErrSignatureInvalid
	}
	auditID, err := s.ports.Ledger.RecordContractSignature(ctx, carecontract.ContractSignatureEntry{
		OrganizationID: organizationID, VersionID: record.Version.ID, SignerAccountID: who.accountID,
		SignerName: signerName, DocumentSHA256: document.SHA256, SignedAt: now, Input: input,
	})
	if err != nil {
		return carecontract.SigningView{}, err
	}
	documentID := document.ID
	if err := s.store.InsertSignature(ctx, organizationID, NewSignature{
		ContractID: record.Summary.ID, VersionID: record.Version.ID, DocumentID: &documentID, Method: input.Method,
		Channel: who.channel, AuditSignatureID: &auditID, SignerName: signerName, SignerAccountID: who.accountID,
		SigningLinkID: who.linkID, VersionSHA256: checksum, DocumentSHA256: document.SHA256, WithdrawalNoticeAt: &now,
		EarlyStartRequested: input.EarlyStart, SignedOn: calendar.DateFromTime(now), SignedAt: now,
	}); err != nil {
		return carecontract.SigningView{}, err
	}
	accountID := int64(0)
	if who.accountID != nil {
		accountID = *who.accountID
	}
	status, err := concludeSigned(ctx, s.store, organizationID, accountID, record, input.Method, now)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	if err := s.event(ctx, organizationID, record, accountID, ports.ActionSign, map[string]any{
		"via": who.channel, "method": input.Method, "document_sha256": document.SHA256, "version_sha256": checksum,
		"early_start": input.EarlyStart, "status": status,
	}); err != nil {
		return carecontract.SigningView{}, err
	}
	view, err := s.view(ctx, organizationID, record, document)
	if err != nil {
		return carecontract.SigningView{}, err
	}
	ends := withdrawalEnd(now)
	view.Status = status
	view.Signature = &carecontract.SigningSignature{
		Method: input.Method, SignerName: signerName, SignedAt: now, EarlyStartRequested: input.EarlyStart, WithdrawalEndsOn: &ends,
	}
	return view, s.confirm(ctx, organizationID, record, document, view, party)
}

// confirm enqueues the confirmation mail: the signed PDF, when and by whom
// it was signed, its checksum and the withdrawal notice on a durable medium
// (§ 312f section 2 BGB).
func (s *Signatures) confirm(ctx context.Context, organizationID int64, record ContractRecord, document StoredDocument,
	view carecontract.SigningView, party carecontract.Party) error {
	if strings.TrimSpace(party.Email) == "" {
		return nil
	}
	mailings, err := s.store.Mailings(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return err
	}
	address := party.Email
	if len(mailings) > 0 {
		address = mailings[0].Recipient
	}
	signature := view.Signature
	_, err = s.ports.Post.EnqueueContractMail(ctx, carecontract.ContractMail{
		OrganizationID: organizationID, ContractID: record.Summary.ID, Recipient: address,
		GuardianName: fullName(party.FirstName, party.LastName), TraegerName: view.TraegerName,
		ChildName: fullName(record.Version.ChildFirstName, record.Version.ChildLastName),
		Modules:   joinGerman(ModuleNames(record.Version.Services)), Filename: document.File.Filename, PDF: document.File.Data,
		IdempotencyKey: fmt.Sprintf("care-contract:%d:signed:%d", record.Summary.ID, record.Version.ID),
		Purpose:        carecontract.MailPurposeSigned, SignerName: signature.SignerName,
		SignedAt: berlinStamp(signature.SignedAt), DocumentSHA256: document.SHA256, EarlyStart: signature.EarlyStartRequested,
		WithdrawalEndsOn: germanDate(*signature.WithdrawalEndsOn), WithdrawalNotice: view.WithdrawalNotice,
	})
	if err != nil {
		return fmt.Errorf("care contract: enqueue confirmation: %w", err)
	}
	return nil
}

// view is what the parents read of a contract.
func (s *Signatures) view(ctx context.Context, organizationID int64, record ContractRecord, document StoredDocument) (carecontract.SigningView, error) {
	letterhead, err := s.ports.Sender.TraegerLetterhead(ctx, organizationID)
	if err != nil {
		return carecontract.SigningView{}, fmt.Errorf("care contract: letterhead: %w", err)
	}
	// A contract is sent only with a named party, so one of both is set.
	traeger := firstText(firstLine(textLines(letterhead)), record.Version.Terms.ContractPartner)
	partner := firstText(record.Version.Terms.ContractPartner, traeger)
	party, _ := recipient(record.Parties)
	version := record.Version
	version.ChildBirthDate = nil
	return carecontract.SigningView{
		ContractID: record.Summary.ID, OrganizationID: organizationID, Status: record.Summary.Status,
		TraegerName: traeger, PartyName: fullName(party.FirstName, party.LastName), Version: version,
		DocumentSHA256: document.SHA256, WithdrawalNotice: withdrawalNotice(partner, letterhead),
	}, nil
}

func (s *Signatures) event(ctx context.Context, organizationID int64, record ContractRecord, accountID int64, action string, details map[string]any) error {
	contractID, schoolID := record.Summary.ID, record.Summary.SchoolID
	if err := s.store.InsertEvents(ctx, organizationID, []Event{{
		ContractID: &contractID, SchoolID: &schoolID, AccountID: accountID, Action: action, Details: details,
	}}); err != nil {
		return fmt.Errorf("care contract: record %s: %w", action, err)
	}
	return nil
}

// berlinStamp prints an instant as the parents read it.
func berlinStamp(at time.Time) string {
	return at.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
}

func (s *Signatures) now() time.Time { return s.clock.now() }
