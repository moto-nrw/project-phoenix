package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ceremony rules moved here with the passkey flows (#3331): which origin
// may start a ceremony, which relying party it runs under and how a stored
// credential is summarised.

func TestValidateTenantPasskeyOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tenantDomain string
		origin       string
		subdomain    string
		wantErr      bool
	}{
		{
			name:         "localhost root accepted",
			tenantDomain: "localhost",
			origin:       "http://localhost:3000",
		},
		{
			name:         "localhost tenant subdomain accepted",
			tenantDomain: "localhost",
			origin:       "http://school.localhost:3000",
			subdomain:    "school",
		},
		{
			name:         "localhost wrong subdomain rejected",
			tenantDomain: "localhost",
			origin:       "http://other.localhost:3000",
			subdomain:    "school",
			wantErr:      true,
		},
		{
			name:         "production tenant subdomain accepted",
			tenantDomain: "moto-app.de",
			origin:       "https://school.moto-app.de",
			subdomain:    "school",
		},
		{
			name:         "production root rejected",
			tenantDomain: "moto-app.de",
			origin:       "https://moto-app.de",
			subdomain:    "school",
			wantErr:      true,
		},
		{
			name:         "production empty subdomain rejected",
			tenantDomain: "moto-app.de",
			origin:       "https://school.moto-app.de",
			wantErr:      true,
		},
		{
			name:         "invalid origin rejected",
			tenantDomain: "localhost",
			origin:       "not-a-url",
			wantErr:      true,
		},
		{
			name:         "unsupported origin scheme rejected",
			tenantDomain: "localhost",
			origin:       "ftp://localhost",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTenantPasskeyOrigin(tt.origin, tt.subdomain, tt.tenantDomain)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrPasskeyOriginInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

// The operator ceremonies are pinned to the operator portal's own host.
func TestValidateOperatorPasskeyOrigin(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateOperatorPasskeyOrigin("https://operator.moto-app.de", "operator.moto-app.de"))
	require.ErrorIs(t, ValidateOperatorPasskeyOrigin("https://school.moto-app.de", "operator.moto-app.de"), ErrPasskeyOriginInvalid)
	require.ErrorIs(t, ValidateOperatorPasskeyOrigin("not-a-url", "operator.moto-app.de"), ErrPasskeyOriginInvalid)
}

func TestPasskeyHostHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "example.com", HostWithoutPort("https://Example.COM:443/ignored"))
	assert.Equal(t, "school.localhost", HostWithoutPort("school.localhost:3000"))

	host, err := OriginHostWithoutPort("http://school.localhost:3000")
	require.NoError(t, err)
	assert.Equal(t, "school.localhost", host)

	// A configured localhost relying party follows the request's own host,
	// so every school subdomain works in development.
	rpID, err := ResolvePasskeyRPIDForOrigin("localhost", "http://school.localhost:3000")
	require.NoError(t, err)
	assert.Equal(t, "school.localhost", rpID)

	rpID, err = ResolvePasskeyRPIDForOrigin("example.com", "https://school.example.com")
	require.NoError(t, err)
	assert.Equal(t, "example.com", rpID)

	assert.Equal(t, "Short name", NormalizePasskeyName("  Short name  "))
	assert.Empty(t, NormalizePasskeyName("   "))
	assert.Len(t, NormalizePasskeyName(strings.Repeat("a", 90)), 80)
}

func TestSummarizePasskeyCredentials(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	lastUsedAt := now.Add(time.Minute)

	account := SummarizeAccountPasskey(AccountPasskeyCredential{
		ID: 44, CreatedAt: now, Name: "Laptop", LastUsedAt: &lastUsedAt,
	})
	assert.Equal(t, "44", account.ID)
	assert.Equal(t, "Laptop", account.Name)
	assert.Equal(t, now, account.CreatedAt)
	assert.Equal(t, &lastUsedAt, account.LastUsedAt)

	operator := SummarizeOperatorPasskey(OperatorPasskeyCredential{
		ID: 7, CreatedAt: now, Name: "YubiKey",
	})
	assert.Equal(t, "7", operator.ID)
	assert.Equal(t, "YubiKey", operator.Name)
	assert.Nil(t, operator.LastUsedAt)
}

// A fresh user handle is minted at full length whenever an identity has no
// credential to take one from.
func TestGeneratePasskeyUserHandle(t *testing.T) {
	t.Parallel()

	handle, err := GeneratePasskeyUserHandle()
	require.NoError(t, err)
	assert.Len(t, handle, PasskeyUserHandleBytes)

	other, err := GeneratePasskeyUserHandle()
	require.NoError(t, err)
	assert.NotEqual(t, handle, other)
}
