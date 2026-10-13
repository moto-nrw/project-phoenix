package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type discardingCommand struct{}

func (discardingCommand) Append(context.Context, any) error { return nil }

// Routing keeps every repository the bundle carries; a field the routed copy
// forgot would silently drop that repository from the service graph.
func TestWithRoutedWritesKeepsEveryRepository(t *testing.T) {
	t.Parallel()

	runtime := Runtime(func(context.Context) (bun.IDB, int64) { return nil, 0 })
	routed := NewRepositories(runtime, runtime).WithRoutedWrites(discardingCommand{})
	value := reflect.ValueOf(routed).Elem()
	for index := 0; index < value.NumField(); index++ {
		name := value.Type().Field(index).Name
		if name == "BookingConsistency" {
			continue // built only with the Enrollment projection
		}
		require.Falsef(t, value.Field(index).IsNil(), "routed repositories dropped %s", name)
	}
}
