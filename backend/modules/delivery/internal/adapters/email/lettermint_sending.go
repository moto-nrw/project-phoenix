package email

import (
	"log/slog"
	"regexp"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/wneessen/go-mail"
)

const (
	headerLettermintRoute    = mail.Header("X-Lettermint-Route")
	headerPreserveMessageID  = mail.Header("X-LM-Preserve-Message-ID")
	lettermintRouteSecurity  = "sicherheit"
	lettermintRouteNotices   = "mitteilungen"
	lettermintPreserveHeader = "true"
)

// lettermintAcceptedID finds the provider's ID in the 250 answer to DATA.
var lettermintAcceptedID = regexp.MustCompile(`(?i)\bid=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\b`)

// withTransportSender applies the sender this transport may use. The display
// name stays: it names the school.
func (m *SMTPMailer) withTransportSender(message mailcontract.Message) mailcontract.Message {
	switch {
	case m.fixedFrom != "":
		message.From.Address = m.fixedFrom
	case !m.lettermint.Enabled():
	case message.Security:
		message.From.Address = m.lettermint.SecurityFrom
	default:
		message.From.Address = m.lettermint.NoticeFrom
	}
	return message
}

// setRouteHeaders names the route on every mail: without it a mail would run
// over Lettermint's undeletable default route.
func (m *SMTPMailer) setRouteHeaders(msg *mail.Msg, message mailcontract.Message) {
	if !m.lettermint.Enabled() {
		return
	}
	route := lettermintRouteNotices
	if message.Security {
		route = lettermintRouteSecurity
	}
	msg.SetGenHeader(headerLettermintRoute, route)
	msg.SetGenHeader(headerPreserveMessageID, lettermintPreserveHeader)
}

// providerMessageID reads the provider's ID of the accepted mail. Other servers
// answer in their own format, which says nothing to moto. Without the ID the mail
// drops out of the reconciliation, which must show. The answer stays out of the
// log line: a server may echo the recipient in it.
func (m *SMTPMailer) providerMessageID(msg *mail.Msg, template string) string {
	if !m.lettermint.Enabled() {
		return ""
	}
	match := lettermintAcceptedID.FindStringSubmatch(msg.ServerResponse())
	if match == nil {
		m.logger.Warn("email accepted without a provider id",
			slog.String("message_id", msg.GetMessageID()),
			slog.String("template", template))
		return ""
	}
	return match[1]
}
