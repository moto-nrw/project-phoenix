package timetablehttp

import (
	"encoding/json"
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// codedFieldError is a request validation failure that names its registered
// code and the form field it is about (#2516). Bind methods return it so the
// client marks the field and words the refusal from the code; the message
// stays the diagnostic text.
type codedFieldError struct {
	code  string
	field string
	msg   string
}

func (e *codedFieldError) Error() string { return e.msg }

// invalidField builds a codedFieldError for a Bind method.
func invalidField(code, field, msg string) error {
	return &codedFieldError{code: code, field: field, msg: msg}
}

// bindErrorRenderer answers a failed render.Bind: a coded field refusal keeps
// its code and field, anything else (a broken body) stays general.input.
func bindErrorRenderer(err error) render.Renderer {
	var coded *codedFieldError
	if errors.As(err, &coded) {
		return common.ErrorInvalidOnField(coded, coded.code, coded.field)
	}
	return common.ErrorInvalidRequest(err)
}

// invalidOnField renders a 400 with code that marks field.
func invalidOnField(code, field, msg string) render.Renderer {
	return common.ErrorInvalidOnField(errors.New(msg), code, field)
}

// invalidWithCode adapts ErrorInvalidRequestWithCode to an ErrorRule.
func invalidWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorInvalidRequestWithCode(err, code) }
}

// conflictWithCode adapts ErrorConflictWithCode to an ErrorRule.
func conflictWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorConflictWithCode(err, code) }
}

// notFoundWithCode adapts ErrorNotFoundWithCode to an ErrorRule.
func notFoundWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorNotFoundWithCode(err, code) }
}

// forbiddenWithCode adapts ErrorForbiddenWithCode to an ErrorRule.
func forbiddenWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorForbiddenWithCode(err, code) }
}

// codedInvalid renders a 400 with the code and values of the CodedError in
// err's chain; an uncoded error stays general.input.
func codedInvalid(err error) render.Renderer {
	coded, ok := timetable.AsCoded(err)
	if !ok {
		return common.ErrorInvalidRequest(err)
	}
	return common.ErrorInvalidRequestWithDetails(err, coded.Code, refusalDetails(coded.Values))
}

// refusalDetails puts the values a refusal names into wire form; nil when
// it names none, so each code carries only its own parameters.
func refusalDetails(values timetable.RefusalValues) map[string]any {
	if values.IsZero() {
		return nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	details := map[string]any{}
	if err := json.Unmarshal(encoded, &details); err != nil {
		return nil
	}
	return details
}

// codedInvalidOnField renders like codedInvalid and marks field.
func codedInvalidOnField(err error, field string) render.Renderer {
	renderer := codedInvalid(err)
	if resp, ok := renderer.(*common.ErrResponse); ok {
		resp.Errors = []common.FieldError{{Field: field, Reason: resp.ErrorText}}
	}
	return renderer
}

// codedOr renders the CodedError in err's chain through codedInvalid and
// otherwise as a 400 with fallbackCode.
func codedOr(fallbackCode string) func(error) render.Renderer {
	return func(err error) render.Renderer {
		if _, ok := timetable.AsCoded(err); ok {
			return codedInvalid(err)
		}
		return common.ErrorInvalidRequestWithCode(err, fallbackCode)
	}
}
