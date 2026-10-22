package peopledirectory

func (s *StudentState) GetTenantID() int64                   { return s.TenantID }
func (s *StudentState) SetTenantID(id int64)                 { s.TenantID = id }
func (s *DirectoryPersonSnapshot) GetTenantID() int64        { return s.TenantID }
func (s *DirectoryPersonSnapshot) SetTenantID(id int64)      { s.TenantID = id }
func (s *GuardianProfileSnapshot) GetTenantID() int64        { return s.TenantID }
func (s *GuardianProfileSnapshot) SetTenantID(id int64)      { s.TenantID = id }
func (s *GuardianProfilePhoneSnapshot) GetTenantID() int64   { return s.TenantID }
func (s *GuardianProfilePhoneSnapshot) SetTenantID(id int64) { s.TenantID = id }
func (s *GuardianRelationshipSnapshot) GetTenantID() int64   { return s.TenantID }
func (s *GuardianRelationshipSnapshot) SetTenantID(id int64) { s.TenantID = id }
func (s *Staff) GetTenantID() int64                          { return s.TenantID }
func (s *Staff) SetTenantID(id int64)                        { s.TenantID = id }
func (s *Teacher) GetTenantID() int64                        { return s.TenantID }
func (s *Teacher) SetTenantID(id int64)                      { s.TenantID = id }
func (s *Guest) GetTenantID() int64                          { return s.TenantID }
func (s *Guest) SetTenantID(id int64)                        { s.TenantID = id }
