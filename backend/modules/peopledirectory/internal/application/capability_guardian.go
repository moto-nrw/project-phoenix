package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

func (s *Service) observed(operation string, fn func() error) error {
	started := time.Now()
	err := fn()
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Err: err})
	return err
}

func (s *Service) guardianProvider() (peopledirectory.GuardianProvider, error) {
	if s.guardianDirectory == nil {
		return nil, peopledirectory.ErrGuardianProviderUnbound
	}
	return s.guardianDirectory, nil
}

func invalidGuardian(reason string) error {
	return &peopledirectory.InvalidGuardianError{Reason: reason}
}
