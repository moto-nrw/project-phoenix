package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
)

// teamChatLeadKey is the seed key of the school admin the product screenshots
// show: the colleagues on shift write to her during the day, so her Team-Chat
// carries the times of the simulated day instead of the seed's.
const teamChatLeadKey = "katarzyna-schmidt"

// messageLog is what the run wrote: the parent conversations by parent
// index and the colleagues who already wrote in the Team-Chat.
type messageLog struct {
	threads     map[int]parentThread
	teamSenders []int64
}

// parentThread is a parent conversation and the parent message in it the OGS
// answers.
type parentThread struct {
	ThreadID, MessageID int64
	ChildID             int64
}

// parentLogin is the parents portal side of a simulation client.
type parentLogin interface {
	LoginParent(email, password string) error
}

type teamChatAction struct{ at wallClock }

func (teamChatAction) Name() string { return "write in the Team-Chat" }

// Run lets a colleague on shift write to the school admin. Who writes and
// what follows the wave: the Frühdienst reports the early care, the late
// shift the first children, midday the arrived classes.
func (a teamChatAction) Run(_ context.Context, rt *Runtime) error {
	lead, ok := rt.teamChatLead()
	if !ok {
		return nil
	}
	sender, body, ok := rt.teamChatNote(a.at, lead.StaffID)
	if !ok {
		return nil
	}
	if err := rt.sendTeamMessage(sender, lead, body); err != nil {
		return err
	}
	rt.messages.teamSenders = append(rt.messages.teamSenders, sender.StaffID)
	rt.Counts.Messages++
	return nil
}

func (rt *Runtime) teamChatLead() (AccountCredentials, bool) {
	accounts := append(slices.Clone(rt.State.Accounts.Admin), rt.State.Accounts.Betreuer...)
	i := slices.IndexFunc(accounts, func(account AccountCredentials) bool { return account.Key == teamChatLeadKey })
	if i < 0 || accounts[i].Name == "" {
		return AccountCredentials{}, false
	}
	return accounts[i], true
}

// teamChatNote picks the colleague and the text of a wave. Only a colleague
// who is stamped in and on shift writes, and each one once.
func (rt *Runtime) teamChatNote(c wallClock, leadID int64) (AccountCredentials, string, bool) {
	switch c {
	case earlyTeamNote:
		if rt.Counts.EarlyCareChildren == 0 {
			return AccountCredentials{}, "", false
		}
		var staff []int64
		for _, block := range rt.Blocks {
			staff = append(staff, block.presentStaff()...)
		}
		sender, _, ok := rt.teamChatSender(c, leadID, staff)
		return sender, "Guten Morgen! Die Frühbetreuung läuft, " + children(rt.Counts.EarlyCareChildren, "schon da") + ".", ok
	case lateShiftCheck:
		sender, supervises, ok := rt.teamChatSender(c, leadID, rt.SupervisorStaffIDs)
		if early := len(rt.Rooms.inRooms(rt.Rooms.sessionRooms)); supervises && early > 0 {
			return sender, "Kurze Info: " + children(early, "schon bei mir") + ", der Unterricht ist ausgefallen.", ok
		}
		return sender, "Kurze Info: Ab 12 Uhr kommen die ersten Klassen.", ok
	default:
		sender, _, ok := rt.teamChatSender(c, leadID, rt.SupervisorStaffIDs)
		return sender, "Alle Klassen sind da, " + children(len(rt.Present), "heute bei uns") + ".", ok
	}
}

// children says how many children are where: "5 Kinder sind schon da".
func children(n int, where string) string {
	if n == 1 {
		return "ein Kind ist " + where
	}
	return fmt.Sprintf("%d Kinder sind %s", n, where)
}

// teamChatSender returns the first preferred colleague who may write, else
// any other; the flag tells which.
func (rt *Runtime) teamChatSender(c wallClock, leadID int64, preferred []int64) (AccountCredentials, bool, bool) {
	accounts := staffAccounts(rt.State.Accounts)
	may := func(id int64) bool {
		_, hasLogin := accounts[id]
		return hasLogin && id != leadID && rt.ClockedInStaff[id] && !rt.StaffOnLeave[id] &&
			rt.onShiftAt(id, c) && !slices.Contains(rt.messages.teamSenders, id)
	}
	for _, id := range preferred {
		if may(id) {
			return accounts[id], true, true
		}
	}
	for _, account := range append(slices.Clone(rt.State.Accounts.Betreuer), rt.State.Accounts.Admin...) {
		if may(account.StaffID) {
			return accounts[account.StaffID], false, true
		}
	}
	return AccountCredentials{}, false, false
}

// sendTeamMessage opens the sender's conversation with the recipient and
// writes into it, as the Team-Chat does (seed_staff_messaging.go).
func (rt *Runtime) sendTeamMessage(sender, recipient AccountCredentials, body string) error {
	client, err := rt.staffClient(sender)
	if err != nil {
		return err
	}
	raw, err := client.Get("/api/staff-messages/recipients")
	if err != nil {
		return fmt.Errorf("list Team-Chat recipients: %w", err)
	}
	var recipients struct {
		Data []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &recipients); err != nil {
		return fmt.Errorf("decode Team-Chat recipients: %w", err)
	}
	i := slices.IndexFunc(recipients.Data, func(row struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
	}) bool {
		return row.Name == recipient.Name
	})
	if i < 0 {
		return fmt.Errorf("Team-Chat recipient %q not found", recipient.Name)
	}
	opened, err := client.Post("/api/staff-messages/threads/open", map[string]any{"account_id": recipients.Data[i].AccountID})
	if err != nil {
		return fmt.Errorf("open Team-Chat conversation: %w", err)
	}
	threadID, _, err := threadMessageIDs(opened)
	if err != nil {
		return fmt.Errorf("decode Team-Chat conversation: %w", err)
	}
	if _, err := client.Post(fmt.Sprintf("/api/staff-messages/threads/%d", threadID), map[string]any{"body": body}); err != nil {
		return fmt.Errorf("write in the Team-Chat: %w", err)
	}
	return nil
}

// threadMessageIDs reads data.thread_id and the ID of the last message of
// data.messages, 0 without messages. The API writes IDs as strings.
func threadMessageIDs(raw []byte) (int64, int64, error) {
	var envelope struct {
		Data struct {
			ThreadID json.Number `json:"thread_id"`
			Messages []struct {
				ID json.Number `json:"id"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, 0, err
	}
	threadID, err := strconv.ParseInt(envelope.Data.ThreadID.String(), 10, 64)
	if err != nil || threadID <= 0 {
		return 0, 0, fmt.Errorf("invalid thread_id %q", envelope.Data.ThreadID)
	}
	if len(envelope.Data.Messages) == 0 {
		return threadID, 0, nil
	}
	messageID, _ := strconv.ParseInt(envelope.Data.Messages[len(envelope.Data.Messages)-1].ID.String(), 10, 64)
	return threadID, messageID, nil
}
