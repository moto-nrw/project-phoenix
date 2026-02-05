package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
)

// seedSuggestions creates sample feedback posts with votes
func (s *Seeder) seedSuggestions(ctx context.Context) error {
	if len(s.result.Accounts) == 0 {
		return fmt.Errorf("accounts required for suggestion authors")
	}

	posts := []struct {
		title       string
		description string
		status      string
	}{
		{
			title:       "Wöchentliche Übersicht der Anwesenheiten",
			description: "Es wäre super hilfreich, wenn man eine wöchentliche Zusammenfassung der Anwesenheiten pro Gruppe als PDF exportieren könnte. Aktuell muss ich das alles manuell zusammenstellen, was sehr zeitaufwendig ist.",
			status:      suggestions.StatusOpen,
		},
		{
			title:       "Benachrichtigung bei vergessener Abmeldung",
			description: "Wenn ein Kind am Ende des Tages nicht abgemeldet wurde, sollte eine automatische Benachrichtigung an die zuständige Betreuungsperson gesendet werden. So vergessen wir niemanden.",
			status:      suggestions.StatusPlanned,
		},
		{
			title:       "Dunkelmodus für die App",
			description: "Am Nachmittag bei schlechtem Licht wäre ein Dark Mode sehr angenehm für die Augen. Viele andere Apps bieten das mittlerweile auch an.",
			status:      suggestions.StatusOpen,
		},
	}

	now := time.Now()

	for i, data := range posts {
		// Rotate through available accounts as authors
		author := s.result.Accounts[i%len(s.result.Accounts)]

		post := &suggestions.Post{
			Title:       data.title,
			Description: data.description,
			AuthorID:    author.ID,
			Status:      data.status,
			Score:       0,
		}
		post.CreatedAt = now.Add(-time.Duration(len(posts)-i) * 24 * time.Hour) // stagger creation dates
		post.UpdatedAt = post.CreatedAt

		_, err := s.tx.NewInsert().Model(post).
			ModelTableExpr("suggestions.posts").
			On("CONFLICT DO NOTHING").
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create suggestion post: %w", err)
		}

		if post.ID == 0 {
			continue // already existed
		}

		s.result.SuggestionPosts = append(s.result.SuggestionPosts, post)

		// Add votes from other accounts
		for j := 1; j <= 3 && j < len(s.result.Accounts); j++ {
			voter := s.result.Accounts[(i+j)%len(s.result.Accounts)]
			direction := suggestions.DirectionUp
			if j == 3 {
				direction = suggestions.DirectionDown
			}

			vote := &suggestions.Vote{
				PostID:    post.ID,
				VoterID:   voter.ID,
				Direction: direction,
			}
			vote.CreatedAt = post.CreatedAt.Add(time.Duration(j) * time.Hour)
			vote.UpdatedAt = vote.CreatedAt

			_, err := s.tx.NewInsert().Model(vote).
				ModelTableExpr("suggestions.votes").
				On("CONFLICT DO NOTHING").
				Exec(ctx)
			if err != nil {
				continue // skip duplicate votes
			}
		}

		// Update score on the post
		_, err = s.tx.NewRaw(`
			UPDATE suggestions.posts SET score = (
				SELECT COALESCE(SUM(CASE WHEN direction = 'up' THEN 1 ELSE -1 END), 0)
				FROM suggestions.votes WHERE post_id = ?
			) WHERE id = ?
		`, post.ID, post.ID).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update post score: %w", err)
		}
	}

	if s.verbose {
		log.Printf("Created %d suggestion posts with votes", len(s.result.SuggestionPosts))
	}

	return nil
}
