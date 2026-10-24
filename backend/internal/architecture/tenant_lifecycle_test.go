package architecture

import (
	"path/filepath"
	"testing"
)

func TestTenantLifecycleReadIsAnExactAdministrativeException(t *testing.T) {
	t.Parallel()
	p, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var grant ReadProjection
	for _, candidate := range p.ReadProjections {
		if candidate.ID == "organization-tenancy.tenant-data-lifecycle" {
			grant = candidate
		}
	}
	pkg := p.packageMap()[p.absolutePackage(grant.Package)]
	if !p.isTenantLifecycleRead(grant, pkg) {
		t.Fatal("approved ADR 0092 read missing")
	}
	for _, change := range []func(*ReadProjection, *Package){
		func(g *ReadProjection, _ *Package) { g.ID = "another-read" },
		func(g *ReadProjection, _ *Package) {
			g.Package = "modules/organizationtenancy/internal/adapters/postgres"
		},
		func(_ *ReadProjection, p *Package) { p.Owner = "identity-access" },
		func(_ *ReadProjection, p *Package) { p.Role = "application" },
	} {
		g, c := grant, pkg
		change(&g, &c)
		if p.isTenantLifecycleRead(g, c) {
			t.Fatal("administrative exception widened")
		}
	}
	base := *p
	base.ReadProjections = nil
	created := map[string]struct{}{p.absolutePackage(grant.Package): {}}
	candidate := *p
	candidate.ReadProjections = []ReadProjection{grant}
	if got := readProjectionLoosenings(&base, &candidate, created, nil); len(got) != 0 {
		t.Fatalf("new isolated adapter rejected: %v", got)
	}
	if got := readProjectionLoosenings(&base, &candidate, nil, nil); len(got) == 0 {
		t.Fatal("existing adapter acquired a new grant")
	}
}

func TestTenantLifecycleDynamicReadNeverAllowsAnotherCallOrWrite(t *testing.T) {
	t.Parallel()
	source := "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport"
	pkg := Package{Owner: "organization-tenancy", Role: "postgres"}
	if !tenantLifecycleDynamicRead(source, pkg, "exportRun.exportTable", "QueryContext") {
		t.Fatal("catalog read rejected")
	}
	for _, method := range []string{"ExecContext", "Exec", "NewRaw", "QueryRowContext"} {
		if tenantLifecycleDynamicRead(source, pkg, "exportRun.exportTable", method) {
			t.Fatalf("accepted %s", method)
		}
	}
	if tenantLifecycleDynamicRead(source, pkg, "anotherMethod", "QueryContext") || tenantLifecycleDynamicRead(source+"/foreign", pkg, "exportRun.exportTable", "QueryContext") {
		t.Fatal("unrelated dynamic SQL accepted")
	}
}
