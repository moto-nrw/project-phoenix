package compose

import (
	"context"
	"errors"
	"testing"
)

type checkoutSettingsResolver struct {
	settingsResolver
	value string
	err   error
}

func (r checkoutSettingsResolver) ResolveString(context.Context, string) (string, error) {
	return r.value, r.err
}

func TestCheckoutTimeUsesResolvedSettingWithoutFallback(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "15:00", "00:00"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			resolved, err := (settings{settings: checkoutSettingsResolver{value: value}}).DailyCheckoutTime(context.Background())
			if err != nil || resolved != value {
				t.Fatalf("resolved checkout time = %q, %v; want %q unchanged", resolved, err, value)
			}
		})
	}
}

func TestCheckoutTimeDoesNotHideResolutionFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("settings unavailable")
	value, err := (settings{settings: checkoutSettingsResolver{err: failure}}).DailyCheckoutTime(context.Background())
	if value != "" || !errors.Is(err, failure) {
		t.Fatalf("resolution failure lost: value=%q err=%v", value, err)
	}
	_, err = (settings{}).DailyCheckoutTime(context.Background())
	if !errors.Is(err, errSettingsNotConfigured) {
		t.Fatalf("missing settings wiring accepted: %v", err)
	}
}
