package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

func TestRFIDCard_Validate(t *testing.T) {
	tests := []struct {
		name    string
		card    *RFIDCard
		wantErr bool
	}{
		{
			name: "valid RFID card",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD1234EF00"},
				Active:        true,
			},
			wantErr: false,
		},
		{
			name: "valid with minimum length (8 chars)",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD1234"},
				Active:        true,
			},
			wantErr: false,
		},
		{
			name: "valid with lowercase (normalized to uppercase)",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "abcdef123456"},
				Active:        true,
			},
			wantErr: false,
		},
		{
			name: "invalid with separators (non-hex chars)",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "AB:CD:12:34:EF:GH"},
				Active:        true,
			},
			wantErr: true, // G and H are not valid hex, fails hex check
		},
		{
			name: "valid with colons (hex only)",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "AB:CD:12:34:EF:00"},
				Active:        true,
			},
			wantErr: false, // After removing colons: ABCD1234EF00
		},
		{
			name: "valid with dashes",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD-1234-EF00"},
				Active:        true,
			},
			wantErr: false, // After removing dashes: ABCD1234EF00
		},
		{
			name: "valid with spaces",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD 1234 EF00"},
				Active:        true,
			},
			wantErr: false, // After removing spaces: ABCD1234EF00
		},
		{
			name: "empty ID",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: ""},
				Active:        true,
			},
			wantErr: true,
		},
		{
			name: "too short (7 chars)",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD123"},
				Active:        true,
			},
			wantErr: true,
		},
		{
			name: "invalid hex characters",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "GHIJ1234KLMN"},
				Active:        true,
			},
			wantErr: true,
		},
		{
			name: "mixed valid and invalid chars",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABC12XYZ"},
				Active:        true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.card.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RFIDCard.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRFIDCard_Validate_Normalization(t *testing.T) {
	tests := []struct {
		name       string
		inputID    string
		expectedID string
	}{
		{
			name:       "uppercase conversion",
			inputID:    "abcdef123456",
			expectedID: "ABCDEF123456",
		},
		{
			name:       "removes colons",
			inputID:    "AB:CD:EF:12:34:56",
			expectedID: "ABCDEF123456",
		},
		{
			name:       "removes dashes",
			inputID:    "ABCD-EF12-3456",
			expectedID: "ABCDEF123456",
		},
		{
			name:       "removes spaces",
			inputID:    "ABCD EF12 3456",
			expectedID: "ABCDEF123456",
		},
		{
			name:       "removes all separators and uppercases",
			inputID:    "ab:cd-ef 12:34-56",
			expectedID: "ABCDEF123456",
		},
		{
			name:       "trims leading/trailing whitespace",
			inputID:    "  ABCDEF123456  ",
			expectedID: "ABCDEF123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &RFIDCard{
				StringIDModel: base.StringIDModel{ID: tt.inputID},
			}

			err := card.Validate()
			if err != nil {
				t.Fatalf("RFIDCard.Validate() unexpected error = %v", err)
			}

			if card.ID != tt.expectedID {
				t.Errorf("RFIDCard.ID = %q, want %q", card.ID, tt.expectedID)
			}
		})
	}
}

func TestRFIDCard_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		card     *RFIDCard
		expected bool
	}{
		{
			name: "active card",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD12345678"},
				Active:        true,
			},
			expected: true,
		},
		{
			name: "inactive card",
			card: &RFIDCard{
				StringIDModel: base.StringIDModel{ID: "ABCD12345678"},
				Active:        false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.IsActive()
			if got != tt.expected {
				t.Errorf("RFIDCard.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRFIDCard_Activate(t *testing.T) {
	card := &RFIDCard{
		StringIDModel: base.StringIDModel{ID: "ABCD12345678"},
		Active:        false,
	}

	if card.IsActive() {
		t.Error("Card should be inactive initially")
	}

	card.Activate()

	if !card.IsActive() {
		t.Error("Card should be active after Activate()")
	}
}

func TestRFIDCard_Deactivate(t *testing.T) {
	card := &RFIDCard{
		StringIDModel: base.StringIDModel{ID: "ABCD12345678"},
		Active:        true,
	}

	if !card.IsActive() {
		t.Error("Card should be active initially")
	}

	card.Deactivate()

	if card.IsActive() {
		t.Error("Card should be inactive after Deactivate()")
	}
}

func TestRFIDCard_LengthBoundaries(t *testing.T) {
	t.Run("exactly minimum length", func(t *testing.T) {
		// MinRFIDCardLength = 8
		card := &RFIDCard{
			StringIDModel: base.StringIDModel{ID: "12345678"},
		}
		err := card.Validate()
		if err != nil {
			t.Errorf("RFIDCard.Validate() should accept %d chars, got error: %v", MinRFIDCardLength, err)
		}
	})

	t.Run("one below minimum", func(t *testing.T) {
		card := &RFIDCard{
			StringIDModel: base.StringIDModel{ID: "1234567"},
		}
		err := card.Validate()
		if err == nil {
			t.Errorf("RFIDCard.Validate() should reject %d chars", MinRFIDCardLength-1)
		}
	})

	t.Run("exactly maximum length", func(t *testing.T) {
		// MaxRFIDCardLength = 64
		maxID := ""
		for i := 0; i < MaxRFIDCardLength; i++ {
			maxID += "A"
		}
		card := &RFIDCard{
			StringIDModel: base.StringIDModel{ID: maxID},
		}
		err := card.Validate()
		if err != nil {
			t.Errorf("RFIDCard.Validate() should accept %d chars, got error: %v", MaxRFIDCardLength, err)
		}
	})

	t.Run("one above maximum", func(t *testing.T) {
		maxID := ""
		for i := 0; i <= MaxRFIDCardLength; i++ {
			maxID += "A"
		}
		card := &RFIDCard{
			StringIDModel: base.StringIDModel{ID: maxID},
		}
		err := card.Validate()
		if err == nil {
			t.Errorf("RFIDCard.Validate() should reject %d chars", MaxRFIDCardLength+1)
		}
	})
}
