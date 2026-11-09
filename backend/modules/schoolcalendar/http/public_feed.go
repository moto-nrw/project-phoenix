package staffcalendarhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// PublicFeeds is the subscription feed a calendar token opens.
type PublicFeeds interface {
	CalendarFeedByToken(ctx context.Context, token string) (filename, content string, err error)
}

// PublicFeed serves the parent and staff iCalendar subscription feeds at
// /public/calendar/{token}. There is no auth: the token in the URL is the
// capability. A token is tried as a parent feed first, then as a staff
// feed. The feed is shown inline and may be cached privately for an hour.
// Without feeds every token is unknown.
func PublicFeed(feeds PublicFeeds) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if feeds == nil {
			common.RenderStatus(w, r, http.StatusNotFound, "")
			return
		}
		token := chi.URLParam(r, "token")
		filename, content, err := feeds.CalendarFeedByToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, calendarService.ErrNotFound) {
				common.RenderStatus(w, r, http.StatusNotFound, "")
				return
			}
			slog.Error("calendar feed failed",
				"error", err.Error(),
			)
			common.RenderStatus(w, r, http.StatusInternalServerError, "")
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", textnorm.ContentDisposition("inline", filename))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = w.Write([]byte(content))
	}
}
