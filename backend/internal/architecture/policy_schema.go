package architecture

// upgradeToolingBase reads the final schema-4 tooling inventory in schema-5
// vocabulary. It is a wire-format upgrade of an already validated immutable
// base, not an ownership or permission exception in the comparison.
//
// Earlier histories containing actual inbound HTTP owners remain historical
// schema-4 policies. In particular, an HTTP owner can never become a tool by
// reading an old policy. Live candidates use only schema 5.
func (p *Policy) upgradeToolingBase() {
	for _, owner := range p.Owners {
		if owner.Kind == "inbound" && (p.ModulePath != "github.com/moto-nrw/project-phoenix" ||
			(owner.ID != "architecture-check" && owner.ID != "dev-tools")) {
			return
		}
	}
	for i := range p.Owners {
		if p.Owners[i].Kind == "inbound" {
			p.Owners[i].Kind = "tool"
		}
	}
	for i := range p.Rules {
		if p.Rules[i].SourceOwnerKind == "inbound" {
			p.Rules[i].SourceOwnerKind = "tool"
		}
		if p.Rules[i].TargetOwnerKind == "inbound" {
			p.Rules[i].TargetOwnerKind = "tool"
		}
	}
	p.SchemaVersion = 5
}
