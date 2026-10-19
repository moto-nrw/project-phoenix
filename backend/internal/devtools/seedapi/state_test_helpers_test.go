package api

import "github.com/moto-nrw/project-phoenix/demoprofile"

func LoadSeedState(path string) (*SeedState, error) {
	return demoprofile.LoadSeedState(path)
}
