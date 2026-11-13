package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// FormLabelsResponse names a request's answers with the questions of the
// form version and legal texts it displayed (#6142). The change request view
// keys its rows by field and consent key; without these it could only show the
// keys. Both lists are empty when the request has neither snapshot nor form
// version.
type FormLabelsResponse struct {
	Fields      []FormLabelField      `json:"fields"`
	LegalBlocks []FormLabelLegalBlock `json:"legal_blocks"`
}

// FormLabelField is one question of the form. Translations carry only the
// label translations that still match the German label.
type FormLabelField struct {
	Key            string                  `json:"key"`
	Label          string                  `json:"label"`
	AppliesToChild bool                    `json:"applies_to_child,omitempty"`
	Options        []FormLabelOption       `json:"options,omitempty"`
	Translations   capability.Translations `json:"translations,omitempty"`
}

// FormLabelOption is the shown label of a stored select value.
type FormLabelOption struct {
	Label        string                  `json:"label"`
	Value        string                  `json:"value"`
	Translations capability.Translations `json:"translations,omitempty"`
}

// FormLabelLegalBlock is the title of a consent the form asked for.
type FormLabelLegalBlock struct {
	Key          string                  `json:"key"`
	Title        string                  `json:"title"`
	Translations capability.Translations `json:"translations,omitempty"`
}

func (rs *Resource) getAdminFormLabels(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var labels *capability.RequestFormLabels
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		loaded, loadErr := rs.RequestService.FormLabels(ctx, id, "")
		labels = loaded
		return loadErr
	})
	if err != nil {
		if errors.Is(err, capability.ErrRequestNotFound) {
			common.RenderError(w, r, renderRequestNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load form labels", err))
		return
	}
	common.Respond(w, r, http.StatusOK, toFormLabelsResponse(labels), "Form labels retrieved")
}

func (rs *Resource) getPublicFormLabels(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}
	var labels *capability.RequestFormLabels
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		loaded, loadErr := rs.RequestService.FormLabels(adminCtx, 0, token)
		labels = loaded
		return loadErr
	})
	if err != nil {
		if errors.Is(err, capability.ErrRequestNotFound) {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load form labels", err))
		return
	}
	common.Respond(w, r, http.StatusOK, toFormLabelsResponse(labels), "Form labels retrieved")
}

// toFormLabelsResponse keeps only what names an answer: info blocks hold no
// answer, and help texts, rules and attachments are form concerns.
func toFormLabelsResponse(labels *capability.RequestFormLabels) FormLabelsResponse {
	out := FormLabelsResponse{Fields: []FormLabelField{}, LegalBlocks: []FormLabelLegalBlock{}}
	if labels == nil {
		return out
	}
	for _, f := range labels.Fields {
		if f.Type == capability.FormFieldInfo {
			continue
		}
		field := FormLabelField{
			Key:            f.Key,
			Label:          f.Label,
			AppliesToChild: f.AppliesToCh,
			Translations:   f.Translations.Fresh(map[string]string{capability.TranslationAttrLabel: f.Label}),
		}
		for _, o := range f.Options {
			field.Options = append(field.Options, FormLabelOption{
				Label:        o.Label,
				Value:        o.Value,
				Translations: o.Translations.Fresh(map[string]string{capability.TranslationAttrLabel: o.Label}),
			})
		}
		out.Fields = append(out.Fields, field)
	}
	for _, b := range labels.LegalBlocks {
		out.LegalBlocks = append(out.LegalBlocks, FormLabelLegalBlock{
			Key:          b.Key,
			Title:        b.Title,
			Translations: titleTranslations(b.Translations),
		})
	}
	return out
}

// titleTranslations keeps only the title of a block's translations; the
// block's own translations are already limited to current ones.
func titleTranslations(all capability.Translations) capability.Translations {
	var out capability.Translations
	for locale, attrs := range all {
		title, ok := attrs[capability.TranslationAttrTitle]
		if !ok || title.Text == "" {
			continue
		}
		if out == nil {
			out = capability.Translations{}
		}
		out[locale] = map[string]capability.TranslatedText{capability.TranslationAttrTitle: {Text: title.Text}}
	}
	return out
}
