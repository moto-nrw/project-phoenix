package simulate

import (
	"context"
	"fmt"
	"strconv"
)

// The parents who write during the day, by their index in the seed state:
// the first parent is the one the product screenshots show, the second shares
// that child. The others write about their own child.
const (
	firstParent      = 0
	morningParent    = 2
	afternoonParent  = 3
	parentPortalPath = "/parent/me/messages/children/%d"
)

type parentQuestionAction struct{}

func (parentQuestionAction) Name() string { return "a parent writes in the morning" }

// Run lets a parent ask the OGS something in the morning; the OGS answers at
// midday (afternoonMessagesAction).
func (parentQuestionAction) Run(_ context.Context, rt *Runtime) error {
	name, childID, ok := rt.parentChild(morningParent)
	if !ok {
		return nil
	}
	body := fmt.Sprintf("Guten Morgen! Ist die Trinkflasche von %s gestern bei Ihnen geblieben?", name)
	thread, err := rt.parentWrites(morningParent, childID, body)
	if err != nil {
		return err
	}
	rt.messages.threads[morningParent] = thread
	return nil
}

type firstParentReplyAction struct{}

func (firstParentReplyAction) Name() string { return "the OGS answers the first parent" }

// Run writes a short note of the OGS into the first parent's conversation, so
// it ends with a recent answer of the school. A child who did not come gets
// none.
func (firstParentReplyAction) Run(_ context.Context, rt *Runtime) error {
	name, childID, ok := rt.parentChild(firstParent)
	if !ok || !rt.Present[childID] {
		return nil
	}
	client, err := rt.parentClient(firstParent)
	if err != nil {
		return err
	}
	raw, err := client.Get(fmt.Sprintf(parentPortalPath, childID))
	if err != nil {
		return fmt.Errorf("read the first parent's conversation: %w", err)
	}
	threadID, _, err := threadMessageIDs(raw)
	if err != nil {
		return fmt.Errorf("decode the first parent's conversation: %w", err)
	}
	return rt.ogsAnswers(parentThread{ThreadID: threadID, ChildID: childID},
		fmt.Sprintf("Kurze Info: %s ist gut bei uns angekommen.", name))
}

type afternoonMessagesAction struct{}

func (afternoonMessagesAction) Name() string { return "parents and the OGS write at midday" }

// Run answers the morning question and lets another parent ask about a later
// day, which stays open for the team.
func (afternoonMessagesAction) Run(_ context.Context, rt *Runtime) error {
	if thread, ok := rt.messages.threads[morningParent]; ok {
		answer := "Ja, die Trinkflasche steht bei uns im Fach. Sie können sie jederzeit abholen."
		if rt.Present[thread.ChildID] {
			answer = fmt.Sprintf("Ja, die Trinkflasche steht bei uns im Fach. %s nimmt sie heute mit.", rt.firstName(thread.ChildID))
		}
		if err := rt.ogsAnswers(thread, answer); err != nil {
			return err
		}
	}
	name, childID, ok := rt.parentChild(afternoonParent)
	if !ok {
		return nil
	}
	_, err := rt.parentWrites(afternoonParent, childID, fmt.Sprintf("Hallo! Kann %s am Freitag schon um 14 Uhr abgeholt werden?", name))
	return err
}

// parentChild returns the first name and ID of the parent's first child,
// false when the seed has no such parent or the child is unknown.
func (rt *Runtime) parentChild(parent int) (string, int64, bool) {
	if parent >= len(rt.State.Parents) || len(rt.State.Parents[parent].StudentIDs) == 0 {
		return "", 0, false
	}
	childID := rt.State.Parents[parent].StudentIDs[0]
	child, ok := rt.child(childID)
	if !ok || child.FirstName == "" {
		return "", 0, false
	}
	return child.FirstName, childID, true
}

// parentClient signs the parent in to the parents portal on a client of its
// own; the client must offer the parents portal login.
func (rt *Runtime) parentClient(parent int) (Client, error) {
	client, err := buildClient(rt.Options.Client, rt.State.BaseURL, rt.Options.Verbose)
	if err != nil {
		return nil, err
	}
	login, ok := client.(parentLogin)
	if !ok {
		return nil, fmt.Errorf("simulation client has no parents portal login")
	}
	credentials := rt.State.Parents[parent]
	if err := login.LoginParent(credentials.Email, credentials.Password); err != nil {
		return nil, fmt.Errorf("parent login %d: %w", parent, err)
	}
	return client, nil
}

// parentWrites sends a parent's message about the child and returns the
// conversation with that message.
func (rt *Runtime) parentWrites(parent int, childID int64, body string) (parentThread, error) {
	client, err := rt.parentClient(parent)
	if err != nil {
		return parentThread{}, err
	}
	raw, err := client.Post(fmt.Sprintf(parentPortalPath, childID), map[string]any{"body": body})
	if err != nil {
		return parentThread{}, fmt.Errorf("parent message: %w", err)
	}
	threadID, messageID, err := threadMessageIDs(raw)
	if err != nil {
		return parentThread{}, fmt.Errorf("decode parent message: %w", err)
	}
	rt.Counts.Messages++
	return parentThread{ThreadID: threadID, MessageID: messageID, ChildID: childID}, nil
}

// ogsAnswers replies in the conversation as the OGS and marks the parent's
// message handled. The school admin of the screenshots writes, the admin of
// the run when the profile has none.
func (rt *Runtime) ogsAnswers(thread parentThread, body string) error {
	reply := map[string]any{"body": body}
	if thread.MessageID != 0 {
		reply["handled_up_to_message_id"] = strconv.FormatInt(thread.MessageID, 10)
	}
	client := rt.Client
	if lead, ok := rt.teamChatLead(); ok && lead.Email != "" {
		var err error
		if client, err = rt.staffClient(lead); err != nil {
			return err
		}
	}
	if _, err := client.Post(fmt.Sprintf("/api/messages/threads/%d", thread.ThreadID), reply); err != nil {
		return fmt.Errorf("OGS reply to a parent: %w", err)
	}
	rt.Counts.Messages++
	return nil
}
