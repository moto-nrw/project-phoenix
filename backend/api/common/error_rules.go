package common

import (
	"errors"

	"github.com/go-chi/render"
)

// Declarative error-rule tables — issue #575 B2.
//
// Before this, ten api packages hand-wrote ErrorRenderer switch statements in
// three shapes: flat errors.Is chains, typed-unwrap (errors.As on a domain
// wrapper, rendering the inner sentinel to strip the "domain: op: ..."
// prefix), and one multi-domain dispatcher (api/iot/common — deliberately NOT
// ported onto these rules; its capacity/conflict types carry a PyrePortal
// wire contract and custom Render implementations).
//
// A package's errors.go shrinks to a rule table:
//
//	var errorRules = []common.ErrorRule{
//	    {education.ErrGroupNotFound, common.ErrorNotFound},
//	    {education.ErrDuplicateGroup, common.ErrorConflict},
//	}
//	var ErrorRenderer = common.UnwrapRenderer[*education.EducationError](errorRules, common.ErrorInternalServer)

// ErrorRule maps one sentinel (matched via errors.Is) — or a Match
// predicate for families of errors, e.g. a model IsValidationError check —
// to a renderer constructor such as ErrorNotFound. Rules are evaluated in
// order; the first match wins, so put more-specific rules first.
type ErrorRule struct {
	Target error            // matched via errors.Is when Match is nil
	Match  func(error) bool // optional predicate alternative to Target
	Render func(error) render.Renderer
}

func (rule ErrorRule) matches(err error) bool {
	if rule.Match != nil {
		return rule.Match(err)
	}
	return errors.Is(err, rule.Target)
}

// RenderWithRules walks rules in order and renders err through the first
// matching rule. The full err (wrapper included) is passed to the rule's
// Render — use UnwrapRenderer when the user-facing text must be the inner
// sentinel instead. No match falls through to fallback.
func RenderWithRules(err error, rules []ErrorRule, fallback func(error) render.Renderer) render.Renderer {
	for _, rule := range rules {
		if rule.matches(err) {
			return rule.Render(err)
		}
	}
	return fallback(err)
}

// RulesRenderer packages RenderWithRules as an ErrorRenderer-shaped function
// for the flat-switch packages (active, feedback, usercontext).
func RulesRenderer(rules []ErrorRule, fallback func(error) render.Renderer) func(error) render.Renderer {
	return func(err error) render.Renderer {
		return RenderWithRules(err, rules, fallback)
	}
}

// UnwrapRenderer builds an ErrorRenderer for the typed-wrapper packages
// (rooms, groups, schedules, activities, users): errors.As to the domain
// wrapper E, match the rules against the wrapped chain, and render the
// INNER sentinel — the JSON `error` field carries just the (German)
// sentinel message instead of the wrapper's "domain: {Op}: ..." prefix.
// The fallback receives the FULL error so 500 logs keep the operation
// context.
//
// A non-E error (or an E whose inner error matches no rule) falls through
// to fallback unchanged — same contract as the hand-written switches this
// replaces.
func UnwrapRenderer[E interface {
	error
	Unwrap() error
}](rules []ErrorRule, fallback func(error) render.Renderer) func(error) render.Renderer {
	return func(err error) render.Renderer {
		var wrapper E
		if errors.As(err, &wrapper) {
			inner := wrapper.Unwrap()
			for _, rule := range rules {
				if rule.matches(inner) {
					return rule.Render(inner)
				}
			}
		}
		return fallback(err)
	}
}
