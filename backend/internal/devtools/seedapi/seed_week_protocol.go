package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedWeekProtocolStep records a demo day's weekly protocols through the
// local Worker endpoint, then closes one through the normal staff endpoint.
// This leaves reviewable day and PDF data without inserting snapshots.
type seedWeekProtocolStep struct{}

func (seedWeekProtocolStep) Name() string { return "Seeding weekly protocols" }

func (seedWeekProtocolStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("weekly protocol seed client is not available")
	}
	if _, err := rt.Client.PostWithHeaders("/api/week-protocols/seed", nil, map[string]string{seedTokenHeader: "true"}); err != nil {
		return fmt.Errorf("record weekly protocol day: %w", err)
	}
	raw, err := rt.Client.Get("/api/week-protocols")
	if err != nil {
		return fmt.Errorf("list weekly protocols: %w", err)
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decode weekly protocol list: %w", err)
	}
	if len(response.Data) == 0 || response.Data[0].ID == "" {
		return fmt.Errorf("weekly protocol seed did not create a protocol")
	}
	if _, err := rt.Client.Post("/api/week-protocols/"+response.Data[0].ID+"/close", nil); err != nil {
		return fmt.Errorf("close weekly protocol: %w", err)
	}
	fmt.Println("  1 Wochenprotokoll mit abgelegtem PDF erstellt")
	return nil
}
