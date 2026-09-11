package seedtoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldExposeInvitationToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		env    string
		host   string
		header string
		want   bool
	}{
		{name: "missing header denies", env: "development", host: "localhost:8080", want: false},
		{name: "development localhost allows", env: "development", host: "localhost:8080", header: "true", want: true},
		{name: "dev loopback allows", env: "dev", host: "127.0.0.1:8080", header: "true", want: true},
		{name: "test ipv6 loopback allows", env: "test", host: "[::1]:8080", header: "true", want: true},
		{name: "local docker service allows", env: "local", host: "server:8080", header: "true", want: true},
		{name: "case-insensitive header allows", env: "development", host: "localhost", header: "TRUE", want: true},
		{name: "production denies", env: "production", host: "localhost:8080", header: "true", want: false},
		{name: "staging denies", env: "staging", host: "localhost:8080", header: "true", want: false},
		{name: "preview denies", env: "preview", host: "localhost:8080", header: "true", want: false},
		{name: "typo denies", env: "developement", host: "localhost:8080", header: "true", want: false},
		{name: "empty env denies", env: "", host: "localhost:8080", header: "true", want: false},
		{name: "public host denies", env: "development", host: "api-staging.moto-app.de", header: "true", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldExposeInvitationToken(tt.header, tt.host, tt.env))
		})
	}
}
