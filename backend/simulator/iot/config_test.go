package iot

import "testing"

func validConfig() *Config {
	return &Config{
		BaseURL: "https://example.com",
		Devices: []DeviceConfig{
			{DeviceID: "device-1", APIKey: "api-key"},
		},
		Event: EventConfig{
			Interval:         minEventInterval,
			MaxEventsPerTick: 1,
			Rotation: RotationConfig{
				Order:     []RotationPhase{RotationPhaseHeimatraum, RotationPhaseAG},
				MinAGHops: 1,
				MaxAGHops: 2,
			},
			Actions: []ActionConfig{
				{Type: ActionCheckIn, Weight: 1},
			},
		},
	}
}

func TestConfigValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "valid https", baseURL: "https://example.com", wantErr: false},
		{name: "valid http", baseURL: "http://localhost:8080", wantErr: false},
		{name: "missing scheme", baseURL: "example.com", wantErr: true},
		{name: "unsupported scheme", baseURL: "ftp://example.com", wantErr: true},
		{name: "missing host", baseURL: "http://", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.BaseURL = tc.baseURL
			err := cfg.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for base_url %q", tc.baseURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for base_url %q: %v", tc.baseURL, err)
			}
		})
	}
}
