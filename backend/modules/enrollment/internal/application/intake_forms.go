package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// errSettingsNotConfigured is the configuration error of an intake composed
// without its settings.
var errSettingsNotConfigured = errors.New("enrollment settings resolver is not configured")

// IsEnrollmentEnabled reports the tenant's enrollment master toggle, so a
// deactivated tenant 404s at the form load instead of at submit. Caller must
// be inside a tenant transaction.
func (s *Intake) IsEnrollmentEnabled(ctx context.Context) bool {
	return s.isEnrollmentEnabled(ctx)
}

func (s *Intake) isEnrollmentEnabled(ctx context.Context) bool {
	if s.deps.Settings == nil {
		return false
	}
	return s.deps.Settings.EnrollmentEnabled(ctx)
}

func (s *Intake) allowSubmissionEdit(ctx context.Context) bool {
	if s.deps.Settings == nil {
		return true
	}
	return s.deps.Settings.AllowSubmissionEdit(ctx)
}

// collectsSchoolClass resolves the effective school-class capability.
func (s *Intake) collectsSchoolClass(ctx context.Context) (bool, error) {
	if s.deps.Settings == nil {
		return false, errSettingsNotConfigured
	}
	collectGrade, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", settingCollectGradeLevel, err)
	}
	collectClass, err := s.deps.Settings.CollectSchoolClass(ctx)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", settingCollectSchoolClass, err)
	}
	return collectGrade && collectClass, nil
}

// formCapabilities resolves the three settings that govern the form's core
// inputs. collect_school_class is ineffective while grade collection is off,
// because a class without its grade is ambiguous.
func (s *Intake) formCapabilities(ctx context.Context) (enrollment.FormCapabilities, error) {
	if s.deps.Settings == nil {
		return enrollment.FormCapabilities{}, errSettingsNotConfigured
	}
	collectGrade, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return enrollment.FormCapabilities{}, fmt.Errorf("resolve %s: %w", settingCollectGradeLevel, err)
	}
	collectClass, err := s.deps.Settings.CollectSchoolClass(ctx)
	if err != nil {
		return enrollment.FormCapabilities{}, fmt.Errorf("resolve %s: %w", settingCollectSchoolClass, err)
	}
	offeringsEnabled, err := s.deps.Settings.CareOfferingsEnabled(ctx)
	if err != nil {
		return enrollment.FormCapabilities{}, fmt.Errorf("resolve %s: %w", settingCareOfferingsEnabled, err)
	}
	return enrollment.FormCapabilities{
		CollectGradeLevel:    collectGrade,
		CollectSchoolClass:   collectGrade && collectClass,
		CareOfferingsEnabled: offeringsEnabled,
	}, nil
}

// effectiveFormCapabilities additionally collects the grade when an active
// offering's availability depends on it.
func effectiveFormCapabilities(capabilities enrollment.FormCapabilities, offerings []*enrollmentModels.CareOffering) enrollment.FormCapabilities {
	if capabilities.CareOfferingsEnabled {
		for _, offering := range offerings {
			if offering != nil && offering.AvailabilityRule.RequiresGradeLevel() {
				capabilities.CollectGradeLevel = true
				break
			}
		}
	}
	capabilities.CollectSchoolClass = capabilities.CollectGradeLevel && capabilities.CollectSchoolClass
	return capabilities
}

// resolveGradeMax reads the server-authoritative grade cap. A missing
// resolver, a read failure or an out-of-range value is a configuration
// error and must not silently change which grades the form accepts.
func (s *Intake) resolveGradeMax(ctx context.Context) (int, error) {
	if s.deps.Settings == nil {
		return 0, errors.New("resolve enrollment.grade_level_max: settings service is not configured")
	}
	value, err := s.deps.Settings.GradeLevelMax(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve enrollment.grade_level_max: %w", err)
	}
	if value < minGradeLevel || value > maxGradeLevel {
		return 0, fmt.Errorf(
			"resolve enrollment.grade_level_max: value %d is outside %d..%d",
			value,
			minGradeLevel,
			maxGradeLevel,
		)
	}
	return value, nil
}

func (s *Intake) resolveStatusTokenExpiry(ctx context.Context) time.Duration {
	const defaultDays = 365
	days := defaultDays
	if s.deps.Settings != nil {
		days = s.deps.Settings.StatusTokenTTLDays(ctx)
	}
	if days <= 0 {
		days = defaultDays
	}
	return time.Duration(days) * 24 * time.Hour
}

// LegalTexts resolves the tenant's legal Markdown and derives the public
// blocks. A resolve failure is propagated, never swallowed: these texts sit
// behind legally relevant blocks, so the endpoint fails rather than let the
// form collect an incomplete legal state.
func (s *Intake) LegalTexts(ctx context.Context) (enrollment.LegalTexts, error) {
	return s.legalTexts(ctx)
}

func (s *Intake) legalTexts(ctx context.Context) (enrollment.LegalTexts, error) {
	if s.deps.Settings == nil {
		return enrollment.LegalTexts{}, nil
	}
	settings, err := s.deps.Settings.LegalSettings(ctx)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	texts := enrollment.LegalTexts{
		AGB:                 strings.TrimSpace(settings.AGB),
		AGBDocumentURL:      strings.TrimSpace(settings.AGBDocumentURL),
		AGBDisplayMode:      legalAGBDisplayMode(strings.TrimSpace(settings.AGBDisplayMode)),
		DSGVO:               strings.TrimSpace(settings.DSGVO),
		EmailContact:        strings.TrimSpace(settings.EmailContact),
		Photo:               strings.TrimSpace(settings.Photo),
		TermsEnabled:        settings.TermsEnabled,
		DSGVOEnabled:        settings.DSGVOEnabled,
		EmailContactEnabled: settings.EmailContactEnabled,
		PhotoEnabled:        settings.PhotoEnabled,
	}
	texts.Blocks = buildLegalBlocks(texts)
	return texts, nil
}

// LegalTextsForPhaseWithLateInvite returns the legal contract of a phase
// behind the public phase gate: the template's enabled blocks win,
// otherwise the tenant-wide ones apply.
func (s *Intake) LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (enrollment.LegalTexts, error) {
	phase, err := s.loadPublicPhaseWithLateInvite(ctx, phaseID, time.Now(), lateInviteToken)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

func (s *Intake) legalTextsForEnrolleePhase(ctx context.Context, phaseID int64, lateInviteToken string, access enrollment.EnrolleeAudienceAccess) (enrollment.LegalTexts, error) {
	phase, err := s.loadEditablePhaseWithLateInvite(ctx, phaseID, time.Now(), lateInviteToken, access)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

func (s *Intake) legalTextsForManualEnrollmentPhase(ctx context.Context, phaseID int64) (enrollment.LegalTexts, error) {
	phase, err := s.loadPhaseForEditableRequest(ctx, phaseID)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

func (s *Intake) legalTextsForLoadedPhase(ctx context.Context, phase *enrollment.Phase) (enrollment.LegalTexts, error) {
	texts, err := s.legalTexts(ctx)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return enrollment.LegalTexts{}, err
	}
	return applyTemplateLegalBlocks(texts, schema), nil
}

// applyTemplateLegalBlocks replaces the settings-derived blocks with the
// template's blocks when at least one of them is enabled: an all-disabled
// template must not erase the tenant's consent contract.
func applyTemplateLegalBlocks(texts enrollment.LegalTexts, schema *enrollment.FormSchema) enrollment.LegalTexts {
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			texts.Blocks = blocks
		}
	}
	return texts
}

// resolveSubmissionLegalBlocks returns the blocks a submission is validated
// and stored against: the template's enabled blocks, otherwise the
// tenant-wide ones.
func (s *Intake) resolveSubmissionLegalBlocks(ctx context.Context, schema *enrollment.FormSchema) ([]enrollment.LegalBlock, error) {
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			return blocks, nil
		}
	}
	texts, err := s.legalTexts(ctx)
	if err != nil {
		return nil, err
	}
	return texts.Blocks, nil
}

func buildLegalBlocks(texts enrollment.LegalTexts) []enrollment.LegalBlock {
	blocks := make([]enrollment.LegalBlock, 0, 4)
	if agbText := legalAGBBlockText(texts); texts.TermsEnabled && agbText != "" {
		blocks = append(blocks, enrollment.LegalBlock{
			Key: enrollmentModels.ConsentKeyAGB, Kind: "terms", Title: "AGB / Teilnahmebedingungen",
			Label: "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
			Text:  agbText, Required: true, SortOrder: 10, Source: enrollment.LegalBlockSourceStandard,
		})
	}
	if texts.DSGVOEnabled && texts.DSGVO != "" {
		blocks = append(blocks, enrollment.LegalBlock{
			Key: enrollmentModels.ConsentKeyDataProcessing, Kind: "privacy_notice", Title: "Datenschutzinformation",
			Label: "Ich habe die Datenschutzinformation der Schule zur Kenntnis genommen.",
			Text:  texts.DSGVO, Required: true, SortOrder: 20, Source: enrollment.LegalBlockSourceStandard,
		})
	}
	if texts.PhotoEnabled && texts.Photo != "" {
		blocks = append(blocks, enrollment.LegalBlock{
			Key: enrollmentModels.ConsentKeyPhoto, Kind: "consent", Title: "Fotoeinwilligung",
			Label: "Mein Kind darf bei Schulveranstaltungen fotografiert werden. Diese Einwilligung ist freiwillig und jederzeit mit Wirkung für die Zukunft widerrufbar.",
			Text:  texts.Photo, Required: false, SortOrder: 30, Source: enrollment.LegalBlockSourceStandard,
		})
	}
	if texts.EmailContactEnabled && texts.EmailContact != "" {
		blocks = append(blocks, enrollment.LegalBlock{
			Key: enrollmentModels.ConsentKeyEmailContact, Kind: "notice", Title: "E-Mail-Kontakt",
			Label: "Die Schule nutzt Ihre E-Mail-Adresse für Rückfragen und Status-Benachrichtigungen zu dieser Anmeldung.",
			Text:  texts.EmailContact, Required: false, SortOrder: 40, Source: enrollment.LegalBlockSourceStandard,
		})
	}
	return blocks
}

func legalAGBDisplayMode(mode string) string {
	if mode == legalAGBDisplayModePDF {
		return legalAGBDisplayModePDF
	}
	return legalAGBDisplayModeText
}

func legalAGBBlockText(texts enrollment.LegalTexts) string {
	if legalAGBDisplayMode(texts.AGBDisplayMode) != legalAGBDisplayModePDF {
		return texts.AGB
	}
	if texts.AGBDocumentURL == "" {
		return ""
	}
	return fmt.Sprintf("Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](%s)", enrollment.PublicEnrollmentLegalDocumentURL(texts.AGBDocumentURL))
}

func buildTemplateLegalBlocks(configured []enrollment.FormLegalBlock) []enrollment.LegalBlock {
	blocks := make([]enrollment.LegalBlock, 0, len(configured))
	for _, block := range configured {
		if !block.Enabled {
			continue
		}
		blocks = append(blocks, enrollment.LegalBlock{
			Key: block.Key, Kind: block.Kind, Title: block.Title, Label: block.Label,
			Text: templateLegalBlockText(block), Required: block.Required, SortOrder: block.SortOrder,
			Source: block.Source, Translations: block.PublicTranslations(),
		})
	}
	// The editor writes blocks in display order, but API-written templates
	// may store them out of order — enforce sort_order at render time.
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].SortOrder < blocks[j].SortOrder
	})
	return blocks
}

func templateLegalBlockText(block enrollment.FormLegalBlock) string {
	if block.Key == enrollmentModels.ConsentKeyAGB &&
		block.DisplayMode == enrollment.LegalBlockDisplayModePDF &&
		strings.TrimSpace(block.DocumentURL) != "" {
		return fmt.Sprintf("Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](%s)", enrollment.PublicEnrollmentLegalDocumentURL(block.DocumentURL))
	}
	return block.Text
}

// loadPhaseForSubmission fetches the selected phase and checks it is active.
// An unknown id is an invalid submission; the window check stays with the
// caller so the handler's error mapping stays specific.
func (s *Intake) loadPhaseForSubmission(ctx context.Context, phaseID int64) (*enrollment.Phase, error) {
	if s.deps.Catalog == nil {
		return nil, fmt.Errorf("submit: phase repo not wired")
	}
	phase, err := s.intakePhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: phase %d not found", enrollment.ErrInvalidSubmission, phaseID)
	}
	if !phase.IsActive {
		return nil, enrollment.ErrEnrollmentDisabled
	}
	return phase, nil
}

func (s *Intake) loadPhaseForEditableRequest(ctx context.Context, phaseID int64) (*enrollment.Phase, error) {
	if !s.isEnrollmentEnabled(ctx) {
		return nil, enrollment.ErrEnrollmentDisabled
	}
	return s.loadPhaseForSubmission(ctx, phaseID)
}

// resolveSubmissionSchema returns the phase's pinned schema, or nil for a
// Basis phase. A Basis phase never inherits the tenant's active schema; a
// pinned schema deleted under the phase submits as Basis too.
func (s *Intake) resolveSubmissionSchema(ctx context.Context, phase *enrollment.Phase) (*enrollment.FormSchema, error) {
	if phase.FormSchemaID == nil {
		return nil, nil
	}
	schema, err := s.intakeSchema(ctx, *phase.FormSchemaID)
	if err == nil {
		return schema, nil
	}
	if s.deps.Runtime.NotFound(err) {
		s.logger().Warn("phase form_schema_id pointed at missing schema; submitting as Basis",
			slog.Int64("phase_id", phase.ID),
			slog.Int64("form_schema_id", *phase.FormSchemaID))
		return nil, nil
	}
	return nil, err
}

// PublicActiveSchema resolves the schema a public parent form renders. A
// Basis phase and a pinned-but-deleted schema return ErrNoActiveSchema, so
// a custom form never leaks into a Basis phase.
func (s *Intake) PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollment.FormSchema, error) {
	phase, err := s.loadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
	if err != nil {
		return nil, err
	}
	if phase.FormSchemaID == nil {
		return nil, enrollment.ErrNoActiveSchema
	}
	schema, err := s.intakeSchema(ctx, *phase.FormSchemaID)
	if err != nil {
		if s.deps.Runtime.NotFound(err) {
			return nil, enrollment.ErrNoActiveSchema
		}
		return nil, err
	}
	return schema, nil
}

// loadPublicPhaseWithLateInvite is the anonymous public form-load gate: it
// rejects every audience-restricted phase.
func (s *Intake) loadPublicPhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollment.Phase, error) {
	return s.loadEditablePhaseWithLateInvite(ctx, phaseID, now, lateInviteToken, enrollment.EnrolleeAudienceAccess{})
}

// loadEditablePhaseWithLateInvite is the shared form-load phase gate. It
// resolves the phase, rejects every restricted audience the caller's access
// does not cover — before the window check, so a restricted phase always
// answers the same 404 — and enforces the window, honouring a valid late
// invite (#1663).
func (s *Intake) loadEditablePhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access enrollment.EnrolleeAudienceAccess) (*enrollment.Phase, error) {
	phase, err := s.loadPhaseForEditableRequest(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	hasValidLateInvite, err := s.hasValidLateInvite(ctx, phaseID, now, lateInviteToken)
	if err != nil {
		return nil, err
	}
	if !hasValidLateInvite && !access.AllowsAudience(phase.Audience) {
		return nil, enrollment.ErrPhaseAudienceRestricted
	}
	if !phase.EnrollmentWindowOpen(now) {
		if strings.TrimSpace(lateInviteToken) == "" {
			return nil, enrollment.ErrEnrollmentWindowClosed
		}
		if !hasValidLateInvite {
			return nil, enrollment.ErrLateInviteInvalid
		}
	}
	// Only offer classes the submit-time eligibility gate will accept. A
	// valid late invite bypasses that gate, so its recipient keeps the full
	// offered list (#1663).
	if hasValidLateInvite {
		clearGradeRestrictionForEligibilityExemptForm(phase)
	} else {
		narrowOfferedClassesToEligibleForForm(phase)
	}
	return phase, nil
}

// hasValidLateInvite resolves a form-load late invite. A valid invite lifts
// the audience restriction and the closed window; an unknown, used or
// expired one grants nothing; a lookup failure is propagated so an outage
// never tells a legitimate recipient their link is invalid (#1663).
func (s *Intake) hasValidLateInvite(ctx context.Context, phaseID int64, now time.Time, token string) (bool, error) {
	if s.deps.LateInvites == nil || strings.TrimSpace(token) == "" {
		return false, nil
	}
	_, err := s.deps.LateInvites.UsableLateInvite(ctx, s.lateInviteTokenHash(token), phaseID, now, false)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, enrollment.ErrLateInviteNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("resolve late invite: %w", err)
	}
}

// clearGradeRestrictionForEligibilityExemptForm drops the grade restriction
// from a form load served to a caller the submit-time eligibility gates do
// not apply to: a valid late invite, or a trusted-source or
// rollover-generated edit draft. Presentation only; the submission reloads
// the phase (#1663).
func clearGradeRestrictionForEligibilityExemptForm(phase *enrollment.Phase) {
	if phase == nil {
		return
	}
	phase.EligibleGradeLevels = []int{}
}

// narrowOfferedClassesToEligibleForForm restricts the offered classes of a
// self-service form load to the eligible ones, so a parent never picks a
// class the submission then rejects. Presentation only (#1663).
func narrowOfferedClassesToEligibleForForm(phase *enrollment.Phase) {
	if phase == nil || len(phase.EligibleSchoolClasses) == 0 {
		return
	}
	eligible := make(map[string]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			eligible[t] = struct{}{}
		}
	}
	narrowed := make([]string, 0, len(phase.AvailableSchoolClasses))
	for _, c := range phase.AvailableSchoolClasses {
		if _, ok := eligible[strings.TrimSpace(c)]; ok {
			narrowed = append(narrowed, c)
		}
	}
	phase.AvailableSchoolClasses = narrowed
	narrowOfferedGradesToEligibleClassesForForm(phase)
}

// narrowOfferedGradesToEligibleClassesForForm derives the form's grade
// options from a class-only eligibility restriction. It only narrows: an
// explicit grade list stays, and a prefixless eligible class keeps every
// grade satisfiable (#1663). Presentation only.
func narrowOfferedGradesToEligibleClassesForForm(phase *enrollment.Phase) {
	if phase == nil || len(phase.EligibleGradeLevels) > 0 {
		return
	}
	grades := make([]int, 0, len(phase.EligibleSchoolClasses))
	seen := make(map[int]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if strings.TrimSpace(c) == "" {
			continue
		}
		level, ok := classGradeLevel(c)
		if !ok {
			// Grade-agnostic eligible class: every grade stays satisfiable.
			return
		}
		if _, dup := seen[level]; dup {
			continue
		}
		seen[level] = struct{}{}
		grades = append(grades, level)
	}
	if len(grades) == 0 {
		return
	}
	sort.Ints(grades)
	phase.EligibleGradeLevels = grades
}

// classGradeLevel parses the grade of a class name; a class without a grade
// number reports false.
func classGradeLevel(class string) (int, bool) {
	prefix := gradePrefix(class)
	if prefix == "" {
		return 0, false
	}
	level, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, false
	}
	return level, true
}
