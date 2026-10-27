package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MailLinkReader returns the first link containing marker in the newest mail
// to recipient received at or after notBefore. The local seed reads Mailpit
// (MAILPIT_URL); the demo sidecar has no mailbox and passes none.
type MailLinkReader func(ctx context.Context, recipient, marker string, notBefore time.Time) (string, error)

// The school's caterer (ADR 0077): a catering business with an address on
// a reserved domain, delivering to the school's serving kitchen.
const (
	seedCatererName            = "Frischeküche Rheinland GmbH"
	seedCatererEmail           = "kontakt@frischekueche-rheinland.example"
	seedCatererDeliveryAddress = "Ausgabeküche im Erdgeschoss, Lindenallee 4, 50667 Köln"
	// seedCatererLinkMarker is the path of the invitation link in the mail:
	// <CATERER_URL>/einladung/<school>/<token>.
	seedCatererLinkMarker = "/einladung/"
)

// seedCatererInvitedAt keeps when the school invited its caterer, so the
// meal order step reads the invitation from the mailbox after it.
const seedCatererInvitedAt = "meal_caterer_invited_at"

// seedMealCatererInvitationStep has the school invite its caterer (ADR
// 0077) right after meal registration is switched on: the outbox mails in
// order, and the parents' invitations that follow would hold the caterer's
// mail back for minutes. The caterer accepts it in seedMealOrdersStep.
// Without a mailbox (the demo sidecar) the school invites nobody: the mail
// would leave for an address nobody reads.
type seedMealCatererInvitationStep struct {
	seeder *Seeder
}

func (seedMealCatererInvitationStep) Name() string { return "Inviting the school's caterer" }

func (s seedMealCatererInvitationStep) Run(_ context.Context, rt *Runtime) error {
	if s.seeder == nil || s.seeder.options.ReadMailLink == nil {
		fmt.Println("  no mailbox to accept a caterer invitation from; school stays without caterer")
		return nil
	}
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("caterer invitation prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	sent := time.Now()
	if _, err := rt.Client.Post("/api/meal-caterer/invitations", map[string]any{
		"email": seedCatererAddress(rt), "caterer_name": seedCatererName, "delivery_address": seedCatererDeliveryAddress,
	}); err != nil {
		return fmt.Errorf("invite the caterer: %w", err)
	}
	rt.Values[seedCatererInvitedAt] = sent
	fmt.Printf("  caterer %s invited\n", seedCatererName)
	return nil
}

// seedMealCaterer accepts the school's invitation the only way ADR 0077
// allows: the link reaches the caterer by mail only, and the caterer opens
// it from its mailbox. It then signs in to its own portal and confirms the
// order of day. Without a mailbox nobody was invited. A mail that does not
// arrive in time leaves the invitation open instead of failing the seed:
// a second seed into the same database queues its mail behind the first
// seed's hundreds, which the outbox sends in order. The coverage ratchet
// still fails when the caterer tables of a fresh seed stay empty.
func seedMealCaterer(ctx context.Context, rt *Runtime, readMailLink MailLinkReader, day seedDate) error {
	if readMailLink == nil {
		return nil
	}
	sent, invited := rt.Values[seedCatererInvitedAt].(time.Time)
	if !invited {
		return fmt.Errorf("the school has not invited its caterer")
	}
	email := seedCatererAddress(rt)
	link, err := readMailLink(ctx, email, seedCatererLinkMarker, sent)
	if err != nil {
		fmt.Printf("  WARNING: caterer invitation mail not read (%v); the invitation stays open\n", err)
		return nil
	}
	_, invitation, found := strings.Cut(link, seedCatererLinkMarker)
	schoolID, token, named := strings.Cut(invitation, "/")
	if !found || !named || schoolID == "" || token == "" || strings.Contains(token, "/") {
		return fmt.Errorf("caterer invitation link %q names no school and token", link)
	}
	// One password for every seed: a caterer already supplying another
	// school of the database confirms the next invitation with it (ADR 0077).
	password := defaultSeedExternalPassword
	if _, err := rt.Client.PostPublic("/caterer/invitations/"+invitation+"/accept", map[string]any{
		"password": password, "confirm_password": password,
	}); err != nil {
		return fmt.Errorf("accept the caterer invitation: %w", err)
	}
	if err := confirmCatererOrder(rt, email, password, schoolID, day); err != nil {
		return err
	}
	fmt.Printf("  caterer %s connected, order of %s confirmed\n", seedCatererName, day)
	return nil
}

// seedCatererAddress is the caterer's address; a scoped seed moves it into
// the school's own domain like every account, so repeated seeds of one
// database never share a caterer account.
func seedCatererAddress(rt *Runtime) string {
	if rt.FixedSeeder != nil && rt.FixedSeeder.accountScope != "" {
		return scopedEmail(seedCatererEmail, rt.FixedSeeder.accountScope)
	}
	return seedCatererEmail
}

// confirmCatererOrder signs in as the caterer and confirms the order of day
// of the school that invited it, among all the schools it supplies.
func confirmCatererOrder(rt *Runtime, email, password, schoolID string, day seedDate) error {
	raw, err := rt.Client.PostPublic("/caterer/auth/login", map[string]any{"email": email, "password": password})
	if err != nil {
		return fmt.Errorf("sign in to the caterer portal: %w", err)
	}
	var session struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &session); err != nil || session.Data.AccessToken == "" {
		return fmt.Errorf("the caterer portal sign-in returned no session")
	}
	auth := AuthRef{Kind: AuthBearer, Label: "caterer", Token: session.Data.AccessToken}
	orderID, err := catererOrderID(rt, auth, schoolID, day)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/caterer/schools/%s/orders/%s/confirm", schoolID, orderID), map[string]any{}); err != nil {
		return fmt.Errorf("confirm the order of %s as the caterer: %w", day, err)
	}
	return nil
}

func catererOrderID(rt *Runtime, auth AuthRef, schoolID string, day seedDate) (string, error) {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/caterer/schools/%s/orders?from=%s&to=%s", schoolID, day, day))
	if err != nil {
		return "", fmt.Errorf("read the caterer's orders of %s: %w", day, err)
	}
	var response struct {
		Data struct {
			Days []struct {
				OrderID string `json:"order_id"`
			} `json:"days"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil || len(response.Data.Days) != 1 || response.Data.Days[0].OrderID == "" {
		return "", fmt.Errorf("the caterer sees no fixed order of %s", day)
	}
	return response.Data.Days[0].OrderID, nil
}
