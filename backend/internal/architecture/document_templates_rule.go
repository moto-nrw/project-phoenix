package architecture

// documentTemplatesTargetPackage is the pure template contract of Document
// Rendering: kinds of document, placeholders, standard templates, the check
// of unknown placeholders and the filling (#4677, ADR 0150).
const documentTemplatesTargetPackage = "modules/documentrendering/templates"

// documentTemplatesSources are the owner roles ADR 0150 lets name the
// template contract: Organization & Tenancy keeps the versions of the
// Träger's templates (public types, port, application, postgres rows, HTTP
// request and error), and the Elternabrechnung fills the template of its
// confirmation letter in its application. Both owners are domains and the
// target is a platform, so no rule can close a cycle (ADR 0058).
var documentTemplatesSources = map[string]map[string]bool{
	"organization-tenancy": {"public": true, "port": true, "application": true, "postgres": true, "http": true},
	"parent-billing":       {"application": true},
}

// reviewedDocumentTemplatesRule is one of the reaches ADR 0150 decides for
// #4677. It is pinned to its source owner and role, the one target package
// and production. It grants no other Document Rendering package, no owner
// kind, no external class and no test scope.
func reviewedDocumentTemplatesRule(rule Rule) bool {
	return documentTemplatesSources[rule.SourceOwner][rule.SourceRole] && rule.SourceOwnerKind == "" && rule.SourcePackage == "" &&
		rule.TargetOwner == "document-rendering" && rule.TargetOwnerKind == "" && rule.TargetRole == "contract" &&
		rule.TargetPackage == documentTemplatesTargetPackage && rule.TargetClass == "" && !rule.SameOwner &&
		len(rule.Scopes) == 1 && rule.Scopes[0] == string(ScopeProduction)
}

// firstPartyAllowedByReviewedDocumentTemplatesRule reports whether the single
// rule that admits this first-party import in the candidate policy is a
// reviewed document templates rule. The import loosening it implies is then
// the intended effect of that rule, not a separate widening.
func firstPartyAllowedByReviewedDocumentTemplatesRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedDocumentTemplatesRule(*decision.Allowed)
}
