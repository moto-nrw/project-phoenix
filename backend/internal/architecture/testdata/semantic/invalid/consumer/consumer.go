package consumer

import "example.test/architecture-semantic/legacy"

func Build() *legacy.Factory {
	return legacy.NewFactory()
}
