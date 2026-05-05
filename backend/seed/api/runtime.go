package api

import "github.com/moto-nrw/project-phoenix/integration/phoenixapi"

type Runtime struct {
	Adapter          *phoenixapi.Adapter
	Client           *Client
	Verbose          bool
	OperatorEmail    string
	OperatorPassword string
	StaffPIN         string
	OperatorAuth     phoenixapi.AuthRef
	TenantAuth       phoenixapi.AuthRef
	Bootstrap        *bootstrapSeedState
	FixedSeeder      *FixedSeeder
	Result           *SeedResult
	State            *SeedState
	// SecondTenant is populated by secondTenantStep when scheduled,
	// then serialised by buildStateStep into seed-state.json under
	// the `second_tenant` key for E2E specs to read.
	SecondTenant *SeedStateSecondTenant
	Values       map[string]any
}

func newRuntime(seeder *Seeder, operatorEmail, operatorPassword, staffPIN string) *Runtime {
	return &Runtime{
		Adapter:          seeder.client.adapter,
		Client:           seeder.client,
		Verbose:          seeder.verbose,
		OperatorEmail:    operatorEmail,
		OperatorPassword: operatorPassword,
		StaffPIN:         staffPIN,
		Result:           &SeedResult{},
		Values:           make(map[string]any),
	}
}

func (r *Runtime) SetOperatorAuth(auth phoenixapi.AuthRef) {
	r.OperatorAuth = auth
	r.Client.BindAuth(auth)
}

func (r *Runtime) SetTenantAuth(auth phoenixapi.AuthRef) {
	r.TenantAuth = auth
	r.Client.BindAuth(auth)
}
