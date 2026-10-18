package careplanhttp

var BindTestConfig func(ResourceConfig) ResourceConfig

func testResource(cfg ResourceConfig) *Resource { return NewResource(BindTestConfig(cfg)) }
