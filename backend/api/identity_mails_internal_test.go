package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The demo environment sends only the demo templates; the owner's demo mails
// must name exactly those.
func TestDemoMailTemplatesAreTheOnesTheDemoMaySend(t *testing.T) {
	t.Parallel()
	assert.Equal(t, email.TemplateDemoAccess, identityaccess.DemoAccessTemplate)
	assert.Equal(t, email.TemplateDemoLead, identityaccess.DemoLeadTemplate)
}
