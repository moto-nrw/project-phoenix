package records

// OperatorWritePolicy validates operator access to a registry definition.
type OperatorWritePolicy interface{ CheckOperatorWritable(string) error }

// CheckOperatorWritable enforces the operator write/reveal access policy: an
// unknown key yields *DefinitionNotFoundError, an AccessAdminOnly key yields
// ErrOperatorAdminOnly, everything else is writable.
func (r *Registry) CheckOperatorWritable(key string) error {
	def := r.GetDefinition(key)
	if def == nil {
		return &DefinitionNotFoundError{Key: key}
	}
	if def.AccessPolicy == AccessAdminOnly {
		return ErrOperatorAdminOnly
	}
	return nil
}
