package api

import (
	"context"
	"fmt"
)

// seedStaffMessagingStep fills the OGS-internal Team-Chat (#2598) with the
// conversations of a working week.
//
// Without it the four users.staff_message_* tables stay empty on every dev
// machine, the Team-Chat screen looks broken to anyone reviewing it, and
// TestSeedCoverageRatchet fails the build for the unlisted empty tables.
//
// The Team-Chat is on by default (opt-out since #3254), so the step relies on
// the registry default instead of writing an override. If that default ever
// flips back, every send below fails with 403 and the seeder stops loudly.
type seedStaffMessagingStep struct{}

func (seedStaffMessagingStep) Name() string { return "Seeding staff messages" }

func (seedStaffMessagingStep) Run(_ context.Context, rt *Runtime) error {
	// rt.State is only assembled by buildStateStep at the very end of the
	// workflow, so the credentials have to come from the fixed seeder - same
	// source seedTimeTrackingHistoryStep uses.
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	// The deferred bind hands the admin session back to later steps.
	defer rt.Client.BindAuth(rt.TenantAuth)

	chat := staffChat{rt: rt, auths: map[int]AuthRef{}}
	threads, sent, unread := 0, 0, 0
	for _, conversation := range staffChatConversations {
		if !chat.hasParticipants(conversation) {
			continue
		}
		if err := chat.seed(conversation); err != nil {
			return err
		}
		threads++
		sent += len(conversation.messages)
		if conversation.leaveUnread {
			unread++
		}
	}
	if threads == 0 {
		return fmt.Errorf("staff messages require the demo staff accounts")
	}
	groups, err := chat.seedGroups()
	if err != nil {
		return err
	}
	fmt.Printf("  %d staff conversations with %d messages created, %d left unread, %d group chats\n", threads, sent, unread, groups)
	return nil
}

// staffChatGroup is one group chat (#4776): creator starts it with the
// members, then the messages follow as (sender, body) in order. Every sender
// reads the group before writing, so the read receipts show a mix: the last
// message stays unread for the members who did not write after it.
type staffChatGroup struct {
	name     string
	creator  int
	members  []int
	messages []staffChatGroupMessage
}

type staffChatGroupMessage struct {
	sender int
	body   string
}

// staffChatGroups are the demo group chats. Katarzyna Schmidt (index 1),
// the admin of the product screenshots, starts the first one.
var staffChatGroups = []staffChatGroup{
	{name: "Frühbetreuung", creator: 1, members: []int{0, 2, 3}, messages: []staffChatGroupMessage{
		{sender: 1, body: "Ab Montag öffnen wir die Frühbetreuung schon um 7:15 Uhr. Wer kann den Dienst übernehmen?"},
		{sender: 0, body: "Montag und Mittwoch kann ich."},
		{sender: 3, body: "Dienstag und Donnerstag übernehme ich."},
		{sender: 1, body: "Danke euch! Den Freitag mache ich selbst."},
	}},
	{name: "Laternenfest", creator: 4, members: []int{1, 5, 6}, messages: []staffChatGroupMessage{
		{sender: 4, body: "Für das Laternenfest brauchen wir noch zwei Leute für den Aufbau ab 15 Uhr."},
		{sender: 5, body: "Ich bin dabei."},
	}},
}

// seedGroups creates the group chats whose members exist in this seed.
func (c staffChat) seedGroups() (int, error) {
	created := 0
	for _, group := range staffChatGroups {
		ok, err := c.seedGroup(group)
		if err != nil {
			return created, err
		}
		if ok {
			created++
		}
	}
	return created, nil
}

func (c staffChat) seedGroup(group staffChatGroup) (bool, error) {
	people := append([]int{group.creator}, group.members...)
	for _, index := range people {
		if _, ok := staffCredentialsByIndex(c.rt.FixedSeeder, index); !ok {
			return false, nil
		}
	}
	threadID, err := c.createGroup(group)
	if err != nil {
		return false, err
	}
	for _, message := range group.messages {
		if err := c.sendToGroup(threadID, message); err != nil {
			return false, err
		}
	}
	return true, nil
}

// createGroup starts the group chat as its creator.
func (c staffChat) createGroup(group staffChatGroup) (string, error) {
	creator, err := c.auth(group.creator)
	if err != nil {
		return "", err
	}
	c.rt.Client.BindAuth(creator)
	memberIDs := make([]string, 0, len(group.members))
	for _, index := range group.members {
		cred, _ := staffCredentialsByIndex(c.rt.FixedSeeder, index)
		id, err := staffAccountID(c.rt, cred)
		if err != nil {
			return "", err
		}
		memberIDs = append(memberIDs, fmt.Sprintf("%d", id))
	}
	raw, err := c.rt.Client.PostWithAuth(creator, "/api/staff-messages/groups", map[string]any{"name": group.name, "account_ids": memberIDs})
	if err != nil {
		return "", fmt.Errorf("create staff group chat %q: %w", group.name, err)
	}
	threadID, err := threadIDFromResponse(raw)
	if err != nil {
		return "", fmt.Errorf("decode POST /api/staff-messages/groups response: %w", err)
	}
	return threadID, nil
}

// sendToGroup reads the group as the sender, then writes.
func (c staffChat) sendToGroup(threadID string, message staffChatGroupMessage) error {
	sender, err := c.auth(message.sender)
	if err != nil {
		return err
	}
	if _, err := c.rt.Client.GetWithAuth(sender, "/api/staff-messages/threads/"+threadID); err != nil {
		return fmt.Errorf("read staff group chat %s: %w", threadID, err)
	}
	if _, err := c.rt.Client.PostWithAuth(sender, "/api/staff-messages/threads/"+threadID, map[string]any{"body": message.body}); err != nil {
		return fmt.Errorf("send to staff group chat %s: %w", threadID, err)
	}
	return nil
}

// staffChatConversation is one direct conversation between two staff
// members, as indices into DemoStaff. The messages alternate, starting with
// first; each sender opens the conversation before answering, as the chat
// does, which is the only way a read cursor is written (#2598: sending marks
// nothing read). The recipient of the last message reads it, unless
// leaveUnread keeps it new for them.
type staffChatConversation struct {
	first, second int
	messages      []string
	leaveUnread   bool
}

// staffChatConversations are the demo team chats. The second staff member,
// Katarzyna Schmidt, is the admin of the product screenshots: she takes part
// in most and has two conversations with a new answer.
var staffChatConversations = []staffChatConversation{
	{first: 0, second: 1, messages: []string{
		"Kannst du heute nach dem Mittagessen die Hausaufgaben der 2a übernehmen? Ich bin bis 14 Uhr im Elterngespräch.",
		"Ja, mache ich. Liegt die Liste im Gruppenraum?",
		"Ja, im roten Ordner auf dem Pult. Danke dir!",
	}},
	{first: 1, second: 2, messages: []string{
		"Die Materialliste für die Bastel-AG liegt im OGS-Büro.",
		"Danke, ich schaue sie mir morgen an.",
		"Gute Besserung, Sabine! Claudia übernimmt heute die Frühbetreuung und die Essensausgabe.",
		"Danke dir! Ich melde mich morgen früh, ob ich wieder fit bin.",
	}, leaveUnread: true},
	{first: 3, second: 1, messages: []string{
		"Ich bin morgen erst ab 11 Uhr da, ich habe einen Arzttermin. Die Essensausgabe und die Busaufsicht schaffe ich.",
		"Alles klar, danke für die Info. Ich trage es im Dienstplan ein.",
	}},
	{first: 1, second: 4, messages: []string{
		"Denkst du an die Laternen für Freitag? Der Kleber ist fast leer.",
		"Ich bringe morgen zwei neue Tuben mit.",
		"Super, danke!",
	}},
	{first: 11, second: 1, messages: []string{
		"Die Eltern aus der Bärengruppe fragen nach dem Termin für das Laternenfest. Steht der schon fest?",
		"Noch nicht ganz, die Schulleitung meldet sich bis Mittwoch.",
		"Okay. Kann ich den Eltern dann schon mal Anfang November sagen?",
	}, leaveUnread: true},
	{first: 8, second: 1, messages: []string{
		"Die Sporthalle ist donnerstags ab 14 Uhr frei. Sollen wir die Fußball-AG dorthin verlegen?",
		"Gute Idee, ich frage die Schulleitung.",
		"Danke, sag mir Bescheid.",
		"Die Schulleitung ist einverstanden, ab nächster Woche geht es los.",
	}},
	{first: 5, second: 6, messages: []string{
		"Hast du die Essensbestellung für nächste Woche schon abgeschickt?",
		"Ja, gestern. Zwei Kinder aus der 1b kommen neu dazu.",
		"Prima, dann trage ich sie in die Essensliste ein.",
	}},
	{first: 12, second: 13, messages: []string{
		"Kannst du morgen meine Lernzeit übernehmen? Ich begleite die Klasse 3a ins Museum.",
		"Klar, ab 13 Uhr bin ich da.",
	}},
}

// staffChat signs every participant in once and keeps the session.
type staffChat struct {
	rt    *Runtime
	auths map[int]AuthRef
}

func (c staffChat) hasParticipants(conversation staffChatConversation) bool {
	_, first := staffCredentialsByIndex(c.rt.FixedSeeder, conversation.first)
	_, second := staffCredentialsByIndex(c.rt.FixedSeeder, conversation.second)
	return first && second
}

func (c staffChat) auth(index int) (AuthRef, error) {
	if auth, ok := c.auths[index]; ok {
		return auth, nil
	}
	cred, _ := staffCredentialsByIndex(c.rt.FixedSeeder, index)
	if err := c.rt.Client.Login(cred.Email, cred.Password); err != nil {
		return AuthRef{}, fmt.Errorf("login as staff message sender %s: %w", cred.Email, err)
	}
	c.auths[index] = c.rt.Client.auth
	return c.rt.Client.auth, nil
}

// seed opens the conversation as first and writes its messages.
func (c staffChat) seed(conversation staffChatConversation) error {
	threadID, err := c.open(conversation)
	if err != nil {
		return err
	}
	senders := []int{conversation.first, conversation.second}
	for i, body := range conversation.messages {
		sender, err := c.auth(senders[i%2])
		if err != nil {
			return err
		}
		if i > 0 {
			if _, err := c.rt.Client.GetWithAuth(sender, "/api/staff-messages/threads/"+threadID); err != nil {
				return fmt.Errorf("read staff conversation %s: %w", threadID, err)
			}
		}
		if _, err := c.rt.Client.PostWithAuth(sender, "/api/staff-messages/threads/"+threadID, map[string]any{"body": body}); err != nil {
			return fmt.Errorf("send staff message to thread %s: %w", threadID, err)
		}
	}
	if conversation.leaveUnread {
		return nil
	}
	reader, err := c.auth(senders[len(conversation.messages)%2])
	if err != nil {
		return err
	}
	if _, err := c.rt.Client.GetWithAuth(reader, "/api/staff-messages/threads/"+threadID); err != nil {
		return fmt.Errorf("read staff conversation %s: %w", threadID, err)
	}
	return nil
}

// open starts the conversation as its first participant. The recipient is
// looked up as the sender: the list leaves out the caller.
func (c staffChat) open(conversation staffChatConversation) (string, error) {
	sender, err := c.auth(conversation.first)
	if err != nil {
		return "", err
	}
	recipient, _ := staffCredentialsByIndex(c.rt.FixedSeeder, conversation.second)
	c.rt.Client.BindAuth(sender)
	recipientID, err := staffAccountID(c.rt, recipient)
	if err != nil {
		return "", err
	}
	raw, err := c.rt.Client.PostWithAuth(sender, "/api/staff-messages/threads/open", map[string]any{"account_id": fmt.Sprintf("%d", recipientID)})
	if err != nil {
		return "", fmt.Errorf("open staff conversation: %w", err)
	}
	threadID, err := threadIDFromResponse(raw)
	if err != nil {
		return "", fmt.Errorf("decode POST /api/staff-messages/threads/open response: %w", err)
	}
	return threadID, nil
}

// staffAccountID resolves the recipient's account id from the Team-Chat's own
// recipient endpoint, matched on the display name the seeder knows. The seed
// state carries staff ids, not account ids, and the chat addresses accounts.
func staffAccountID(rt *Runtime, cred StaffCredentials) (int64, error) {
	body, err := rt.Client.Get("/api/staff-messages/recipients")
	if err != nil {
		return 0, fmt.Errorf("list staff message recipients: %w", err)
	}
	var envelope struct {
		Data []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return 0, fmt.Errorf("decode GET /api/staff-messages/recipients response: %w", err)
	}
	for _, row := range envelope.Data {
		if row.Name != cred.Name {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(row.AccountID, "%d", &id); err != nil {
			return 0, fmt.Errorf("decode account id for staff message recipient %s: %w", cred.Email, err)
		}
		return id, nil
	}
	return 0, fmt.Errorf("staff message recipient account not found for %s", cred.Email)
}

// threadIDFromResponse pulls thread_id out of the {status,data,message} envelope.
func threadIDFromResponse(body []byte) (string, error) {
	var envelope struct {
		Data struct {
			ThreadID string `json:"thread_id"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return "", err
	}
	if envelope.Data.ThreadID == "" {
		return "", fmt.Errorf("thread_id missing")
	}
	return envelope.Data.ThreadID, nil
}
