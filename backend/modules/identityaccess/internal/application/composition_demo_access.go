package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	restoredPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

func NewDemoAccessBinding(dbReady bool, sessions ports.DemoSessions, deps *restoredPorts.DemoDependencies, store ports.DemoAccessStore, execute ports.DemoAdminTx) (*identityaccess.DemoAccess, error) {
	if deps == nil {
		return nil, nil
	}
	if !dbReady || sessions == nil || deps.Schools == nil || deps.Mail == nil || deps.NewToken == nil || deps.Fingerprint == nil {
		return nil, errors.New("identity access compose: every demo access dependency is required")
	}
	flows, err := NewDemoAccess(DemoAccessDependencies{
		Sessions: sessions, Store: store, Schools: DemoSchoolsPort{Schools: deps.Schools},
		Tokens:  DemoAccessTokens{Mint: deps.NewToken, FingerprintFunc: deps.Fingerprint},
		Mail:    DemoAccessMail{Mail: deps.Mail},
		AdminTx: execute,
	})
	if err != nil {
		return nil, err
	}
	return identityaccess.NewDemoAccess(DemoAccessEngine{Flows: flows}), nil
}
