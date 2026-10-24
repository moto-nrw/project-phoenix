package api

import (
	"fmt"
	"strconv"
)

// seedParentGroupThreads writes two group threads (#4784) to the OGS group
// of the demo parent's child: an open one the parent reads and answers, and
// a closed one. The answer lands in the child's conversation with the OGS.
func seedParentGroupThreads(rt *Runtime, auth AuthRef, studentID int64) error {
	groupID, err := seedStudentGroupID(rt, studentID)
	if err != nil {
		return err
	}
	label, err := seedGroupLabel(rt, groupID)
	if err != nil {
		return err
	}
	open, err := seedGroupThread(rt, groupID, label, "Sommerfest am Freitag",
		"Am Freitag feiern wir ab 14 Uhr unser Sommerfest. Wer kann einen Kuchen mitbringen?")
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/group-threads/%d/messages", open), map[string]any{
		"body": "Bitte geben Sie Ihrem Kind auch eine Kopfbedeckung mit.",
	}); err != nil {
		return fmt.Errorf("seed second group message: %w", err)
	}
	closed, err := seedGroupThread(rt, groupID, label, "Elternabend verschoben",
		"Der Elternabend findet erst am Dienstag statt. Die Uhrzeit bleibt gleich.")
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/group-threads/%d/close", closed), map[string]any{}); err != nil {
		return fmt.Errorf("close seeded group thread: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/parent/me/messages/group-threads/%d", open)); err != nil {
		return fmt.Errorf("read seeded group thread: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/messages/children/%d", studentID), map[string]any{
		"body": "Wir bringen gerne einen Apfelkuchen mit.", "group_thread_id": strconv.FormatInt(open, 10),
	}); err != nil {
		return fmt.Errorf("seed group thread answer: %w", err)
	}
	return nil
}

func seedGroupLabel(rt *Runtime, groupID int64) (string, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/groups/%d", groupID))
	if err != nil {
		return "", fmt.Errorf("load group %d: %w", groupID, err)
	}
	var resp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil || resp.Data.Name == "" {
		return "", fmt.Errorf("parse group %d: %w", groupID, err)
	}
	return "Gruppe " + resp.Data.Name, nil
}

func seedGroupThread(rt *Runtime, groupID int64, label, title, body string) (int64, error) {
	raw, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/messages/group-threads", map[string]any{
		"target_type": "group", "target_id": strconv.FormatInt(groupID, 10), "target_label": label,
		"title": title, "body": body,
	})
	if err != nil {
		return 0, fmt.Errorf("seed group thread %q: %w", title, err)
	}
	var resp struct {
		Data struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse group thread %q: %w", title, err)
	}
	return strconv.ParseInt(resp.Data.Thread.ID, 10, 64)
}
