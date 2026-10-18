package peopledirectory

import "time"

func (s *StudentState) GetID() any                              { return s.ID }
func (s *StudentState) GetCreatedAt() time.Time                 { return s.CreatedAt }
func (s *StudentState) GetUpdatedAt() time.Time                 { return s.UpdatedAt }
func (s *StudentState) GetTenantID() int64                      { return s.TenantID }
func (s *StudentState) SetTenantID(id int64)                    { s.TenantID = id }
func (s *DirectoryPersonSnapshot) GetID() any                   { return s.ID }
func (s *DirectoryPersonSnapshot) GetCreatedAt() time.Time      { return s.CreatedAt }
func (s *DirectoryPersonSnapshot) GetUpdatedAt() time.Time      { return s.UpdatedAt }
func (s *DirectoryPersonSnapshot) GetTenantID() int64           { return s.TenantID }
func (s *DirectoryPersonSnapshot) SetTenantID(id int64)         { s.TenantID = id }
func (s *GuardianProfileSnapshot) GetID() any                   { return s.ID }
func (s *GuardianProfileSnapshot) GetCreatedAt() time.Time      { return s.CreatedAt }
func (s *GuardianProfileSnapshot) GetUpdatedAt() time.Time      { return s.UpdatedAt }
func (s *GuardianProfileSnapshot) GetTenantID() int64           { return s.TenantID }
func (s *GuardianProfileSnapshot) SetTenantID(id int64)         { s.TenantID = id }
func (s *GuardianProfilePhoneSnapshot) GetID() any              { return s.ID }
func (s *GuardianProfilePhoneSnapshot) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *GuardianProfilePhoneSnapshot) GetUpdatedAt() time.Time { return s.UpdatedAt }
func (s *GuardianProfilePhoneSnapshot) GetTenantID() int64      { return s.TenantID }
func (s *GuardianProfilePhoneSnapshot) SetTenantID(id int64)    { s.TenantID = id }
func (s *GuardianRelationshipSnapshot) GetID() any              { return s.ID }
func (s *GuardianRelationshipSnapshot) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *GuardianRelationshipSnapshot) GetUpdatedAt() time.Time { return s.UpdatedAt }
func (s *GuardianRelationshipSnapshot) GetTenantID() int64      { return s.TenantID }
func (s *GuardianRelationshipSnapshot) SetTenantID(id int64)    { s.TenantID = id }
func (s *Staff) GetID() any                                     { return s.ID }
func (s *Staff) GetCreatedAt() time.Time                        { return s.CreatedAt }
func (s *Staff) GetUpdatedAt() time.Time                        { return s.UpdatedAt }
func (s *Staff) GetTenantID() int64                             { return s.TenantID }
func (s *Staff) SetTenantID(id int64)                           { s.TenantID = id }
func (s *Teacher) GetID() any                                   { return s.ID }
func (s *Teacher) GetCreatedAt() time.Time                      { return s.CreatedAt }
func (s *Teacher) GetUpdatedAt() time.Time                      { return s.UpdatedAt }
func (s *Teacher) GetTenantID() int64                           { return s.TenantID }
func (s *Teacher) SetTenantID(id int64)                         { s.TenantID = id }
func (s *Guest) GetID() any                                     { return s.ID }
func (s *Guest) GetCreatedAt() time.Time                        { return s.CreatedAt }
func (s *Guest) GetUpdatedAt() time.Time                        { return s.UpdatedAt }
func (s *Guest) GetTenantID() int64                             { return s.TenantID }
func (s *Guest) SetTenantID(id int64)                           { s.TenantID = id }
