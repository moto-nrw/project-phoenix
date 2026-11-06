package architecture

// demoUploadsTargetPackage is the one Delivery package the dev tools binary
// may bind: the local object store whose tenant namespaces the school erasure
// already removes (ADR 0092).
const demoUploadsTargetPackage = "modules/delivery/objects"

// reviewedDemoUploadsRule is the reach ADR 0148 proposes for #5544: the demo
// process, the dev tools binary cmd/devtools (dev-tools/compose, ADR 0060),
// binds the demo server's uploads store so Organization & Tenancy can remove
// the files of retired demo schools. It is pinned to the source owner and
// role, the one target package and production. It grants no other Delivery
// package, no owner kind, no external class and no test scope.
func reviewedDemoUploadsRule(rule Rule) bool {
	return rule.SourceOwner == "dev-tools" && rule.SourceOwnerKind == "" && rule.SourceRole == "compose" && rule.SourcePackage == "" &&
		rule.TargetOwner == "delivery-platform" && rule.TargetOwnerKind == "" && rule.TargetRole == "public" &&
		rule.TargetPackage == demoUploadsTargetPackage && rule.TargetClass == "" && !rule.SameOwner &&
		len(rule.Scopes) == 1 && rule.Scopes[0] == string(ScopeProduction)
}
