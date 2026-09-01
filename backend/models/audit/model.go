package audit

import "github.com/moto-nrw/project-phoenix/models/base"

// Model is the conventional identity/timestamp shape used by audit read
// models.
type Model = base.Model

// TenantModel carries the tenant ID for tenant-scoped audit models.
type TenantModel = base.TenantModel
