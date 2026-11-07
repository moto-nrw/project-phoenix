package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The conclusion of a signed contract (#4679). A signature, online or on
// paper, concludes the sent contract through the one transition of #4680
// (concludeContract): it becomes aktiv with the method and the instant of
// the signature, its version wirksam, under the row lock the caller holds.

// concludeSigned concludes the sent contract of record with the signature
// made at at: einfach_elektronisch for a drawn or typed signature, papier
// for a paper contract. A contract the transition refuses is no contract a
// signature can conclude.
func concludeSigned(ctx context.Context, store Store, organizationID, accountID int64, record ContractRecord,
	signature string, at time.Time) (string, error) {
	method := carecontract.MethodElectronic
	if signature == carecontract.SignaturePaper {
		method = carecontract.MethodPaper
	}
	err := concludeContract(ctx, store, organizationID, accountID, record, carecontract.Concluded{Method: method, At: at})
	if errors.Is(err, carecontract.ErrContractNotConcludable) {
		return "", fmt.Errorf("%w: %w", carecontract.ErrContractNotSignable, err)
	}
	if err != nil {
		return "", err
	}
	return carecontract.StatusActive, nil
}

// versionChecksum is the SHA-256 of the structured version the parents
// sign: child, place of care, services, price, terms and dates. What the
// timeline derives is left out: the state changes when the version takes
// effect, and ValidUntil when a later version starts (#4681).
func versionChecksum(fingerprint func([]byte) string, version carecontract.Version) (string, error) {
	version.Status, version.ValidUntil = "", nil
	encoded, err := json.Marshal(version)
	if err != nil {
		return "", fmt.Errorf("care contract: encode version: %w", err)
	}
	return fingerprint(encoded), nil
}

// withdrawalEnd is the last day of the 14-day withdrawal period that starts
// with the conclusion of a distance contract (§ 355 section 2 BGB); a last
// day on a Saturday, Sunday or nationwide holiday moves to the next working
// day (§ 193 BGB), the day the withdrawal and termination page names too
// (#4682).
func withdrawalEnd(signedAt time.Time) calendar.Date {
	return nextWorkingDay(calendar.DateFromTime(signedAt).AddDays(carecontract.WithdrawalDays))
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
