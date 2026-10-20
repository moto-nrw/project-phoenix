//go:build testhooks

package compose

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

func (d *Directory) ParentFixtureProtocol() peopledirectory.ParentFixtureProtocol {
	return d.Owner.(peopledirectory.ParentFixtureProtocols).ParentFixtureProtocol()
}
