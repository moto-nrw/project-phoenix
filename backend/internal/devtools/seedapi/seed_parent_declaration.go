package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// seedParentDeclarationStep runs after the parent letter: the Erklärungen
// (#3430) reach the same demo guardians.
type seedParentDeclarationStep struct{}

func (seedParentDeclarationStep) Name() string { return "Seeding parent declarations" }

func (seedParentDeclarationStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentDeclaration(rt)
}

// seedDeclarationTitle names the seeded zoo Erklärung; the parent steps and
// the product screenshot eltern-erklaerung-nachweis find it by it.
const seedDeclarationTitle = "Einverständnis: Ausflug in den Zoo"

// seedSwimDeclarationTitle names the second Erklärung. The first family
// leaves it open: a product screenshot opens its signing pad.
const seedSwimDeclarationTitle = "Einverständnis: Schwimmkurs im Hallenbad"

type seedDeclaration struct {
	title, body string
	attachment  string
	document    []byte
	// action decides what the i-th responding family answers: "" for
	// nothing, "agreed", "declined", or "revoked" (agreed, then revoked).
	action func(i int) string
}

// seedParentDeclaration publishes two Erklärungen (#3430), both signed by
// drawing (#4785). The first parent consents to the zoo outing and reads it;
// about two thirds of the other families answer it, a few decline and one
// revokes. The swimming course stays open for the first family, and some
// other families answer it.
func seedParentDeclaration(rt *Runtime) error {
	outing := seedOutingDate(todaySeedDate())
	declarations := []seedDeclaration{
		{
			title: seedDeclarationTitle,
			body: fmt.Sprintf("Liebe Eltern, am %s fahren wir mit der Gruppe in den Zoo. "+
				"Wir treffen uns um 8:30 Uhr auf dem Schulhof und sind gegen 15 Uhr zurück. "+
				"Bitte geben Sie Ihrem Kind einen Rucksack mit Trinkflasche und Regenjacke mit. "+
				"Bitte sagen Sie uns hier, ob Ihr Kind mitfahren darf.", germanDate(outing)),
			attachment: "Einverständnis Zoo.pdf",
			document: demoPDF("Einverständnis Ausflug in den Zoo", []string{
				"Tag: " + germanDate(outing) + ".",
				"Treffpunkt: 8:30 Uhr auf dem Schulhof.",
				"Rückkehr: gegen 15:00 Uhr an der Schule.",
				"Bitte geben Sie Ihrem Kind etwas zu trinken und eine Regenjacke mit.",
			}),
			action: zooDeclarationAction,
		},
		{
			title: seedSwimDeclarationTitle,
			body: "Liebe Eltern, ab nächstem Monat gehen wir donnerstags mit den Kindern ins Hallenbad. " +
				"Eine Schwimmlehrerin leitet den Kurs, zwei Betreuerinnen fahren mit. " +
				"Bitte unterschreiben Sie, ob Ihr Kind teilnehmen darf.",
			action: swimDeclarationAction,
		},
	}
	for _, declaration := range declarations {
		if err := publishSeedDeclaration(rt, declaration); err != nil {
			return err
		}
	}
	if len(rt.Parents) == 0 || rt.Adapter == nil {
		fmt.Printf("  %d parent declarations published\n", len(declarations))
		return nil
	}
	if err := seedDeclarationAnswer(rt); err != nil {
		return err
	}
	for _, declaration := range declarations {
		answered, err := seedDeclarationAnswers(rt, declaration)
		if err != nil {
			return err
		}
		fmt.Printf("  %q published, %d more children answered\n", declaration.title, answered)
	}
	return nil
}

// zooDeclarationAction lets most families agree, a few decline and one
// revoke its consent.
func zooDeclarationAction(i int) string {
	switch {
	case i == 7:
		return "revoked"
	case i%13 == 6:
		return "declined"
	default:
		return "agreed"
	}
}

// swimDeclarationAction lets every third family answer the newer
// Erklärung, so it is still filling up.
func swimDeclarationAction(i int) string {
	switch {
	case i%3 != 0:
		return ""
	case i%15 == 12:
		return "declined"
	default:
		return "agreed"
	}
}

func publishSeedDeclaration(rt *Runtime, declaration seedDeclaration) error {
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":                         declaration.title,
		"body":                          declaration.body,
		"priority":                      "important",
		"delivery_mode":                 "declaration",
		"declaration_kind":              "consent",
		"declaration_signers":           "any",
		"declaration_revocable":         true,
		"declaration_requires_password": false,
		// Agreeing needs a drawn signature (#4785), so the proof, the report
		// and the parent view show one on every machine.
		"declaration_requires_signature": true,
		"response_deadline":              time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
		"send_email":                     true,
		"targets":                        []map[string]any{{"target_type": "school_all"}},
	})
	if err != nil {
		return fmt.Errorf("create parent declaration %q: %w", declaration.title, err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent declaration response: %w", err)
	}
	// The document belongs to the frozen version, so it is attached before
	// the first publication (after that the attachments are fixed).
	if declaration.document != nil {
		if _, err := rt.Client.PostFile(fmt.Sprintf("/api/announcement-attachments/%d", id), "file", declaration.attachment, declaration.document); err != nil {
			return fmt.Errorf("attach file to parent declaration: %w", err)
		}
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent declaration %q: %w", declaration.title, err)
	}
	return nil
}

type seedDeclarationItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
	Declaration *struct {
		Version struct {
			ID string `json:"id"`
		} `json:"version"`
		Children []struct {
			StudentID string  `json:"student_id"`
			CanSubmit bool    `json:"can_submit"`
			MyAction  *string `json:"my_action"`
		} `json:"children"`
	} `json:"declaration"`
}

func seedDeclarationFor(rt *Runtime, auth AuthRef, title string) (*seedDeclarationItem, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/news")
	if err != nil {
		return nil, fmt.Errorf("load parent feed for declaration: %w", err)
	}
	var feed struct {
		Data []seedDeclarationItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &feed); err != nil {
		return nil, fmt.Errorf("parse parent feed for declaration: %w", err)
	}
	for i := range feed.Data {
		if feed.Data[i].Title == title && feed.Data[i].Declaration != nil {
			return &feed.Data[i], nil
		}
	}
	return nil, nil
}

// seedDeclarationAnswer lets the first demo parent consent for their first
// child through the parent API, exactly as the portal does, and marks the
// Erklärung read, so the parents portal lists it as done.
func seedDeclarationAnswer(rt *Runtime) error {
	defer rt.Client.BindAuth(rt.TenantAuth)
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent to declare: %w", err)
	}
	item, err := seedDeclarationFor(rt, auth, seedDeclarationTitle)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("seeded parent declaration is not answerable by the first demo parent")
	}
	for _, child := range item.Declaration.Children {
		if !child.CanSubmit {
			continue
		}
		if err := submitSeedDeclaration(rt, auth, item, child.StudentID, "agreed", parentSignName(rt.seedVisitor(), parent)); err != nil {
			return err
		}
		if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+item.ID+"/read", map[string]any{
			"published_at": item.PublishedAt,
		}); err != nil {
			return fmt.Errorf("mark parent declaration read: %w", err)
		}
		return nil
	}
	return errors.New("seeded parent declaration is not answerable by the first demo parent")
}

// seedDeclarationAnswers answers an Erklärung as the responding families.
// Children of the first family stay unanswered, and so does every child
// someone already answered for.
func seedDeclarationAnswers(rt *Runtime, declaration seedDeclaration) (int, error) {
	defer rt.Client.BindAuth(rt.TenantAuth)
	firstFamily := firstFamilyChildren(rt)
	answered := 0
	for i, parent := range respondingFamilies(rt) {
		action := declaration.action(i)
		if action == "" {
			continue
		}
		count, err := seedDeclarationAnswerAs(rt, parent, declaration.title, action, firstFamily)
		if err != nil {
			return answered, err
		}
		answered += count
	}
	return answered, nil
}

func seedDeclarationAnswerAs(rt *Runtime, parent ParentCredentials, title, action string, firstFamily map[string]bool) (int, error) {
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return 0, fmt.Errorf("login parent to declare: %w", err)
	}
	item, err := seedDeclarationFor(rt, auth, title)
	if err != nil || item == nil {
		return 0, err
	}
	count := 0
	for _, child := range item.Declaration.Children {
		if firstFamily[child.StudentID] || !child.CanSubmit || child.MyAction != nil {
			continue
		}
		if err := answerSeedDeclarationChild(rt, auth, item, child.StudentID, action, parent.Name); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// answerSeedDeclarationChild answers for one child; "revoked" first agrees
// and then takes the consent back.
func answerSeedDeclarationChild(rt *Runtime, auth AuthRef, item *seedDeclarationItem, studentID, action, signer string) error {
	if action != "revoked" {
		return submitSeedDeclaration(rt, auth, item, studentID, action, signer)
	}
	if err := submitSeedDeclaration(rt, auth, item, studentID, "agreed", signer); err != nil {
		return err
	}
	return submitSeedDeclaration(rt, auth, item, studentID, "revoked", signer)
}

// submitSeedDeclaration sends one answer; an agreement carries the drawn
// signature of the parent.
func submitSeedDeclaration(rt *Runtime, auth AuthRef, item *seedDeclarationItem, studentID, action, signer string) error {
	body := map[string]any{"student_id": studentID, "action": action, "version_id": item.Declaration.Version.ID}
	if action == "agreed" {
		body["signature"] = map[string]any{"method": "drawn", "image_png": demoSignaturePNG(signer)}
	}
	if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+item.ID+"/declaration", body); err != nil {
		return fmt.Errorf("declare %s for %q: %w", action, item.Title, err)
	}
	return nil
}
