//go:build testhooks

package compose

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/parentfixture"

func (d *Directory) ParentFixtureProtocol() parentfixture.ParentFixtureProtocol {
	return d.Owner.(parentfixture.ParentFixtureProtocols).ParentFixtureProtocol()
}
