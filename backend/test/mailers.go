package test

import "github.com/moto-nrw/project-phoenix/modules/delivery/emailtest"

// The mail test doubles live with the Delivery Platform (emailtest); these
// aliases keep the suites that take them from the fixture catalog unchanged.
type EmailMessage = emailtest.Message
type Mailer = emailtest.Mailer
type CapturingMailer = emailtest.CapturingMailer
type ReplyToResolverStub = emailtest.ReplyToResolverStub

var NewCapturingMailer = emailtest.NewCapturingMailer
