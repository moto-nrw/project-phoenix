package api

type Runtime struct {
	Adapter          Adapter
	Client           *Client
	Verbose          bool
	OperatorEmail    string
	OperatorPassword string
	StaffPIN         string
	OperatorAuth     AuthRef
	TenantAuth       AuthRef
	Bootstrap        *bootstrapSeedState
	FixedSeeder      *FixedSeeder
	Result           *SeedResult
	State            *SeedState
	Parents          []ParentCredentials
	Enrollment       SeedEnrollmentState
	CareWithdrawals  *SeedCareWithdrawalDemo
	Values           map[string]any
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

func (r *Runtime) SetOperatorAuth(auth AuthRef) {
	r.OperatorAuth = auth
	r.Client.BindAuth(auth)
}

func (r *Runtime) SetTenantAuth(auth AuthRef) {
	r.TenantAuth = auth
	r.Client.BindAuth(auth)
}
