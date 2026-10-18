package devicefleet

import (
	"context"
	"testing"
	"time"
)

func TestOnlineWindowFromMinutes(t *testing.T) {
	t.Parallel()
	if OnlineWindowFromMinutes(nil) != nil {
		t.Fatal("a missing resolver must stay missing so the owner default applies")
	}
	widened, unset, negative := 15, 0, -3
	for _, tc := range []struct {
		minutes int
		want    time.Duration
	}{
		{minutes: widened, want: 15 * time.Minute},
		{minutes: unset, want: 0},
		{minutes: negative, want: 0},
	} {
		minutes := tc.minutes
		window := OnlineWindowFromMinutes(func(context.Context) int { return minutes })
		if got := window(context.Background()); got != tc.want {
			t.Errorf("window for %d minutes = %v, want %v", minutes, got, tc.want)
		}
	}
}
