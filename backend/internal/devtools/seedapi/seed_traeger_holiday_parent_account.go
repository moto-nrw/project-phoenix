package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// The parent account at the holiday registration (#6673, ADR 0216). After
// the siblings, whom the first seeded parent registered with the proof of the
// prefill, the same parent registers a holiday guest once more for both weeks
// of the period: a week the waiting list holds keeps that registration
// received with a waitlisted week. A guest family without a school then
// registers and asks for a parent account; the seed redeems the setup link of
// the offer, which the answer hands the local seeder alone, and signs the
// account in to the parents portal at the Träger. Both registrations show in
// the Eltern-App of their account. The offer mail itself waits behind the
// seed's other parent mails, so the seed does not read it.

const (
	// seedHolidayParentAccountKey names the Träger parent account in the
	// local seed state.
	seedHolidayParentAccountKey = "ferien-elternkonto"
	// seedHolidayGuestFamilyEmail is the invented address of the guest family
	// that sets up its parent account through the offer.
	seedHolidayGuestFamilyEmail = "ferien.gastfamilie@example.org"
	// seedHolidayProofFamilyEmail is the address the seeded parent types when
	// registering a holiday guest with the account's proof.
	seedHolidayProofFamilyEmail = "ferien.nachweis@example.org"
	// seedHolidaySetupLinkMarker is the path of the setup link:
	// <PARENTS_URL>/ferienkonto/<token>.
	seedHolidaySetupLinkMarker = "/ferienkonto/"
)

// seedHolidayParentAccountStep seeds both ways to the parent account.
type seedHolidayParentAccountStep struct{ seeder *Seeder }

func (seedHolidayParentAccountStep) Name() string {
	return "Seeding parent accounts of the holiday registration"
}

func (s seedHolidayParentAccountStep) Run(ctx context.Context, rt *Runtime) error {
	published, ok := rt.Values[seedHolidayLegalPublishedKey].([]byte)
	if !ok || rt.Values[seedHolidayRegistrationsSkippedKey] == true || len(rt.Parents) == 0 {
		fmt.Println("  Ferienanmeldung: no published period or no parent, no parent account")
		return nil
	}
	token, base, err := seedHolidayRegistration(published, 0)
	if err != nil {
		return err
	}
	if err := seedHolidayRegistrationWithAccountProof(ctx, rt, token, base); err != nil {
		return err
	}
	setupURL, err := seedHolidayGuestAskingForAccount(rt, token, base)
	if err != nil || setupURL == "" {
		return err
	}
	credentials, err := seedHolidayParentSetup(ctx, rt, setupURL)
	if err != nil {
		return err
	}
	// Only a local seed keeps the login in its state, like the other local
	// review accounts.
	if s.seeder != nil && (s.seeder.client == nil || localSeedAccountsEnabled(s.seeder.client.baseURL, s.seeder.options)) {
		s.seeder.holidayParentAccount = &credentials
	}
	return nil
}

// seedHolidayGuestRegistration is the base registration with one holiday
// guest booking both weeks, for the guardian named.
func seedHolidayGuestRegistration(base map[string]any, guardian map[string]any, firstName string) map[string]any {
	registration := maps.Clone(base)
	delete(registration, "second_guardian")
	children := base["children"].([]map[string]any)
	guest := maps.Clone(children[1])
	guest["first_name"], guest["birth_date"] = firstName, "2019-06-03"
	guest["last_name"] = guardian["last_name"]
	var bookings []map[string]any
	for _, child := range children {
		bookings = append(bookings, child["bookings"].([]map[string]any)...)
	}
	guest["bookings"] = seedDistinctWeekBookings(bookings)
	registration["children"] = []map[string]any{guest}
	registration["guardian"] = guardian
	registration["declaration_name"] = fmt.Sprintf("%v %v", guardian["first_name"], guardian["last_name"])
	return registration
}

// seedDistinctWeekBookings keeps the first booking of every week.
func seedDistinctWeekBookings(bookings []map[string]any) []map[string]any {
	var weeks []any
	var distinct []map[string]any
	for _, booking := range bookings {
		if slices.Contains(weeks, booking["week_id"]) {
			continue
		}
		weeks = append(weeks, booking["week_id"])
		distinct = append(distinct, booking)
	}
	return distinct
}

// seedHolidayPublicSubmit posts a registration from an address of its own,
// so the seed's many registrations stay below the per-address limit of the
// link. refused reports a refusal the demo server may give (captcha, limit).
// headers adds to the request, such as the seed header.
func seedHolidayPublicSubmit(rt *Runtime, token string, registration map[string]any, clientIP string, headers map[string]string) (raw []byte, refused bool, err error) {
	all := map[string]string{"X-Forwarded-For": clientIP}
	maps.Copy(all, headers)
	raw, err = rt.Client.PostPublicWithHeaders("/traeger/holiday-registrations/"+token, registration, all)
	var refusal *APIError
	if errors.As(err, &refusal) && slices.Contains(seedHolidayRefusals, refusal.Code) {
		fmt.Printf("  Ferienanmeldung: registration refused (%s)\n", refusal.Code)
		return nil, true, nil
	}
	return raw, false, err
}

// seedHolidayRegistrationWithAccountProof registers a holiday guest of the
// first seeded parent with the account's proof from the prefill and checks
// that the Eltern-App lists the parent's registrations.
func seedHolidayRegistrationWithAccountProof(ctx context.Context, rt *Runtime, token string, base map[string]any) error {
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("sign in the parent for the holiday account proof: %w", err)
	}
	proof, err := seedHolidayAccountProof(rt, auth, token)
	if err != nil {
		return err
	}
	// The proof links the account whatever address the form names. An
	// address of its own keeps the registration out of the daily limit of
	// the seeded parent's address, which every demo school of the Träger
	// shares (#6726); its own client address keeps it out of the limit of
	// the seed's address.
	guardian := map[string]any{"first_name": "Sabine", "last_name": "Ferienkind", "email": seedHolidayProofFamilyEmail, "phone": "0151 0000001"}
	if first, last, ok := strings.Cut(strings.TrimSpace(parent.Name), " "); ok && first != "" && last != "" {
		guardian["first_name"], guardian["last_name"] = first, last
	}
	registration := seedHolidayGuestRegistration(base, guardian, "Emil")
	registration["parent_account_proof"] = proof
	_, refused, err := seedHolidayPublicSubmit(rt, token, registration, "198.51.100.210", nil)
	if err != nil {
		return fmt.Errorf("register with the parent account's proof: %w", err)
	}
	if refused {
		return nil
	}
	entries, err := seedHolidayRegistrationEntries(rt, auth)
	if err != nil {
		return err
	}
	if entries < 2 {
		return fmt.Errorf("the Eltern-App lists %d holiday registration(s) of the seeded parent, want the siblings and the guest", entries)
	}
	fmt.Printf("  Eltern-App: %d holiday registrations linked to the seeded parent\n", entries)
	return nil
}

// seedHolidayAccountProof reads the sealed proof of the account from the
// prefill of the link (ADR 0216 point 2).
func seedHolidayAccountProof(rt *Runtime, auth AuthRef, token string) (string, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/holiday-registrations/"+token+"/family")
	if err != nil {
		return "", fmt.Errorf("read the holiday prefill: %w", err)
	}
	var envelope struct {
		Data struct {
			AccountProof string `json:"account_proof"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode the holiday prefill: %w", err)
	}
	if envelope.Data.AccountProof == "" {
		return "", errors.New("the holiday prefill carries no proof of the parent account")
	}
	return envelope.Data.AccountProof, nil
}

// seedHolidayRegistrationEntries counts the holiday registrations the
// Eltern-App lists for the signed-in account.
func seedHolidayRegistrationEntries(rt *Runtime, auth AuthRef) (int, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/holiday-registration-entries")
	if err != nil {
		return 0, fmt.Errorf("read the holiday registrations of the Eltern-App: %w", err)
	}
	var envelope struct {
		Data struct {
			Registrations []json.RawMessage `json:"registrations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("decode the holiday registrations of the Eltern-App: %w", err)
	}
	return len(envelope.Data.Registrations), nil
}

// seedHolidayGuestAskingForAccount registers the guest family, which asks
// for a parent account at the end of the form, and returns the setup link of
// the offer. The answer carries it for the local seeder only (the seed
// header on a local host in a development or demo environment); a refused
// registration returns none.
func seedHolidayGuestAskingForAccount(rt *Runtime, token string, base map[string]any) (string, error) {
	guardian := map[string]any{"first_name": "Jana", "last_name": "Gastfamilie", "email": seedHolidayGuestFamilyEmail, "phone": "0151 0000006"}
	registration := seedHolidayGuestRegistration(base, guardian, "Ole")
	registration["parent_account_requested"] = true
	raw, refused, err := seedHolidayPublicSubmit(rt, token, registration, "198.51.100.211", map[string]string{seedTokenHeader: "true"})
	if err != nil {
		return "", fmt.Errorf("register the guest family asking for a parent account: %w", err)
	}
	if refused {
		return "", nil
	}
	var envelope struct {
		Data struct {
			SetupURL string `json:"parent_account_setup_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode the guest family's registration: %w", err)
	}
	if envelope.Data.SetupURL == "" {
		return "", errors.New("the registration asking for a parent account answered the seeder without the setup link")
	}
	fmt.Println("  Ferienanmeldung: a guest family asks for a parent account")
	return envelope.Data.SetupURL, nil
}

// seedHolidayParentSetup redeems the setup link of the offer with the seed's
// password for external accounts, signs the new account in to the parents
// portal at the Träger, which claims the guest family's registration, and
// returns its credentials for the local seed state.
func seedHolidayParentSetup(ctx context.Context, rt *Runtime, setupURL string) (AccountCredentials, error) {
	_, setupToken, ok := strings.Cut(setupURL, seedHolidaySetupLinkMarker)
	if !ok || setupToken == "" {
		return AccountCredentials{}, errors.New("the offer of a parent account carries no setup link")
	}
	setupToken, _, _ = strings.Cut(setupToken, "?")
	path := "/auth/holiday-parent-setups/" + setupToken
	if _, err := rt.Client.GetPublic(path); err != nil {
		return AccountCredentials{}, fmt.Errorf("open the setup link of the parent account: %w", err)
	}
	// One password for every seed, like the caterer's: an address that
	// already has an account from an earlier run keeps its own.
	password := defaultSeedExternalPassword
	if _, err := rt.Client.PostPublic(path, map[string]any{"password": password, "confirm_password": password}); err != nil {
		return AccountCredentials{}, fmt.Errorf("set up the parent account at the Träger: %w", err)
	}
	auth, err := rt.Adapter.LoginParent(ctx, seedHolidayGuestFamilyEmail, password)
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("sign in the parent account at the Träger: %w", err)
	}
	entries, err := seedHolidayRegistrationEntries(rt, auth)
	if err != nil {
		return AccountCredentials{}, err
	}
	if entries == 0 {
		return AccountCredentials{}, errors.New("the sign-in at the Träger claimed no holiday registration of the guest family")
	}
	fmt.Printf("  Eltern-App: the guest family set up its account at the Träger and sees %d holiday registration(s)\n", entries)
	return AccountCredentials{Key: seedHolidayParentAccountKey, Email: seedHolidayGuestFamilyEmail, Password: password, Name: "Jana Gastfamilie"}, nil
}

// seedHolidayWithParentProof adds the first seeded parent's proof from the
// prefill to the siblings' registration (ADR 0216 point 2), so it is linked
// to the parent's account at once and shows in the Eltern-App. A run without
// seeded parents, or a prefill without a proof, sends it as before; the
// copies of the registration for other families never carry it.
func seedHolidayWithParentProof(ctx context.Context, rt *Runtime, token string, registration map[string]any) map[string]any {
	if len(rt.Parents) == 0 || rt.Adapter == nil {
		return registration
	}
	auth, err := rt.Adapter.LoginParent(ctx, rt.Parents[0].Email, rt.Parents[0].Password)
	if err != nil {
		return registration
	}
	proof, err := seedHolidayAccountProof(rt, auth, token)
	if err != nil {
		return registration
	}
	submitted := maps.Clone(registration)
	submitted["parent_account_proof"] = proof
	return submitted
}
