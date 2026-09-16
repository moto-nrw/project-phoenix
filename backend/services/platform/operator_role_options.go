package platform

import authModels "github.com/moto-nrw/project-phoenix/models/auth"

// OperatorRoleOption is the role projection used by operator role pickers.
// IDs remain decimal strings in JSON so frontend code never loses int64
// precision while parsing a response.
type OperatorRoleOption struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

// OperatorRoleOptions projects auth roles at the service boundary, keeping
// operator HTTP handlers independent of the identity-access persistence model.
func OperatorRoleOptions(roles []*authModels.Role) []OperatorRoleOption {
	options := make([]OperatorRoleOption, 0, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		options = append(options, OperatorRoleOption{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem})
	}
	return options
}
