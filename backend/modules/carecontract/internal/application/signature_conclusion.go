package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The conclusion of a signed contract (#4679). A signature, online or on
// paper, concludes the sent contract: it becomes aktiv and its version
// wirksam, under the row lock the caller holds. #4680 concludes a contract
// at the acceptance of the application with the same transition
// (concludeContract there, methods einfach_elektronisch and papier for the
// signature); once both are merged, concludeSigned calls it.

// concludeSigned moves the sent contract of record to aktiv.
func concludeSigned(ctx context.Context, store SignatureStore, organizationID int64, record ContractRecord, at time.Time) (string, error) {
	if err := store.ConcludeContract(ctx, organizationID, record.Summary.ID, record.Version.ID, at); err != nil {
		return "", err
	}
	return carecontract.StatusActive, nil
}

// versionChecksum is the SHA-256 of the structured version the parents
// sign: child, place of care, services, price, terms and dates. The state
// and the number of the version are left out; they change when it takes
// effect.
func versionChecksum(fingerprint func([]byte) string, version carecontract.Version) (string, error) {
	version.Status = ""
	encoded, err := json.Marshal(version)
	if err != nil {
		return "", fmt.Errorf("care contract: encode version: %w", err)
	}
	return fingerprint(encoded), nil
}

// withdrawalEnd is the last day of the 14-day withdrawal period that starts
// with the conclusion of a distance contract (§ 355 section 2 BGB).
func withdrawalEnd(signedAt time.Time) calendar.Date {
	return calendar.DateFromTime(signedAt).AddDays(14)
}

// withdrawalNotice is the notice of the right of withdrawal (Art. 246a § 1
// section 2 EGBGB, model of its annex 1) in plain language. The parents read
// it before they sign online and get it again in the confirmation mail.
func withdrawalNotice(partner, letterhead string) []string {
	address := ""
	if lines := textLines(letterhead); len(lines) > 0 {
		address = " Die Anschrift: " + joinLines(lines) + "."
	}
	return []string{
		"Widerrufsrecht: Sie können diesen Vertrag innerhalb von 14 Tagen ohne Angabe von Gründen widerrufen. Die Frist beginnt an dem Tag, an dem Sie den Vertrag unterschreiben.",
		"So widerrufen Sie: Teilen Sie " + partner + " eindeutig mit, dass Sie den Vertrag widerrufen. Das geht zum Beispiel per Brief oder E-Mail." + address +
			" Es reicht, wenn Sie die Mitteilung vor dem Ende der Frist absenden.",
		"Folgen des Widerrufs: Sie bekommen alle Zahlungen spätestens 14 Tage nach Ihrem Widerruf zurück, auf demselben Weg, auf dem Sie gezahlt haben. Dafür entstehen Ihnen keine Kosten.",
		"Haben Sie verlangt, dass die Betreuung schon in der Widerrufsfrist beginnt? Dann zahlen Sie für die Betreuung bis zu Ihrem Widerruf einen angemessenen Teil des Beitrags.",
	}
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += ", "
		}
		result += line
	}
	return result
}
