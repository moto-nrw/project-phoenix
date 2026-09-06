package application

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/ports"
)

const retention = 30 * 24 * time.Hour

type WithinAdmin func(context.Context, func(context.Context) error) error

type Service struct {
	store       ports.Store
	access      ports.AccessResolver
	tokens      ports.TokenCodec
	withinAdmin WithinAdmin
	frontendURL *url.URL
	now         func() time.Time
}

func New(store ports.Store, access ports.AccessResolver, tokens ports.TokenCodec, withinAdmin WithinAdmin, frontendURL string, now func() time.Time) (*Service, error) {
	if store == nil || access == nil || tokens == nil || withinAdmin == nil || now == nil {
		return nil, errors.New("request feed: all dependencies are required")
	}
	parsed, err := url.Parse(frontendURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("request feed: FRONTEND_URL must be an absolute URL")
	}
	return &Service{store: store, access: access, tokens: tokens, withinAdmin: withinAdmin, frontendURL: parsed, now: now}, nil
}

func (s *Service) Status(ctx context.Context, tenantID, accountID int64) (requestfeed.Status, error) {
	if _, err := s.requireAccess(ctx, tenantID, accountID); err != nil {
		return requestfeed.Status{}, err
	}
	active, err := s.store.Active(ctx, tenantID, accountID)
	return requestfeed.Status{Active: active}, err
}

func (s *Service) Provision(ctx context.Context, tenantID, accountID int64) (requestfeed.Created, error) {
	access, err := s.requireAccess(ctx, tenantID, accountID)
	if err != nil {
		return requestfeed.Created{}, err
	}
	raw, hash, err := s.tokens.New()
	if err != nil {
		return requestfeed.Created{}, err
	}
	created, err := s.store.Create(ctx, tenantID, accountID, hash)
	if err != nil {
		return requestfeed.Created{}, err
	}
	if !created {
		return requestfeed.Created{}, requestfeed.ErrAlreadyActive
	}
	return requestfeed.Created{URL: s.feedURL(raw, access.Subdomain)}, nil
}

func (s *Service) Rotate(ctx context.Context, tenantID, accountID int64) (requestfeed.Created, error) {
	access, err := s.requireAccess(ctx, tenantID, accountID)
	if err != nil {
		return requestfeed.Created{}, err
	}
	raw, hash, err := s.tokens.New()
	if err != nil {
		return requestfeed.Created{}, err
	}
	updated, err := s.store.Rotate(ctx, tenantID, accountID, hash)
	if err != nil {
		return requestfeed.Created{}, err
	}
	if !updated {
		return requestfeed.Created{}, requestfeed.ErrNotFound
	}
	return requestfeed.Created{URL: s.feedURL(raw, access.Subdomain)}, nil
}

func (s *Service) ByToken(ctx context.Context, raw string) (requestfeed.Feed, error) {
	var result requestfeed.Feed
	err := s.withinAdmin(ctx, func(adminCtx context.Context) error {
		subscription, found, err := s.store.Resolve(adminCtx, s.tokens.Hash(raw))
		if err != nil {
			return err
		}
		if !found {
			return requestfeed.ErrNotFound
		}
		access, err := s.access.Resolve(adminCtx, subscription.TenantID, subscription.AccountID)
		if err != nil {
			return err
		}
		if !access.Allowed() {
			return requestfeed.ErrNotFound
		}
		subscription.SchoolName = access.SchoolName
		subscription.Subdomain = access.Subdomain
		items, err := s.store.List(adminCtx, subscription.TenantID, s.now().UTC().Add(-retention), access)
		if err != nil {
			return err
		}
		result.XML, err = s.render(subscription, items)
		return err
	})
	return result, err
}

func (s *Service) requireAccess(ctx context.Context, tenantID, accountID int64) (domain.Access, error) {
	access, err := s.access.Resolve(ctx, tenantID, accountID)
	if err != nil {
		return domain.Access{}, err
	}
	if !access.Allowed() {
		return domain.Access{}, requestfeed.ErrNotFound
	}
	return access, nil
}

func (s *Service) feedURL(token, subdomain string) string {
	base := *s.frontendURL
	if subdomain != "" {
		host := base.Hostname()
		if host != "localhost" && !strings.HasPrefix(host, subdomain+".") {
			host = subdomain + "." + host
		} else if host == "localhost" {
			host = subdomain + ".localhost"
		}
		if port := base.Port(); port != "" {
			base.Host = host + ":" + port
		} else {
			base.Host = host
		}
	}
	base.Path = "/api/request-feed/" + token
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func (s *Service) requestURL(subdomain string) string {
	base := *s.frontendURL
	feed := s.feedURL("placeholder", subdomain)
	parsed, _ := url.Parse(feed)
	base.Host = parsed.Host
	base.Path = "/anfragen"
	base.RawQuery = "tab=eltern"
	return base.String()
}

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func (s *Service) render(subscription domain.Subscription, items []domain.Item) (string, error) {
	link := s.requestURL(subscription.Subdomain)
	values := make([]rssItem, 0, len(items))
	for _, item := range items {
		label := kindLabel(item.Kind)
		values = append(values, rssItem{
			Title:       "Neue Anfrage: " + label,
			Link:        link,
			Description: fmt.Sprintf("Für %s ist eine neue Elternanfrage eingegangen. Öffnen Sie moto, um sie zu prüfen.", subscription.SchoolName),
			GUID:        rssGUID{IsPermaLink: false, Value: fmt.Sprintf("urn:moto:parent-request:%d:%s:%d", subscription.TenantID, item.Kind, item.ID)},
			PubDate:     item.CreatedAt.UTC().Format(time.RFC1123Z),
		})
	}
	document := rss{Version: "2.0", Channel: rssChannel{
		Title: "Neue Elternanfragen – " + subscription.SchoolName,
		Link:  link, Description: "Hinweise auf neue Elternanfragen. Bearbeiten Sie die Anfragen in moto.", Language: "de-de", Items: values,
	}}
	encoded, err := xml.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("render request feed: %w", err)
	}
	return xml.Header + string(encoded), nil
}

func kindLabel(kind string) string {
	switch kind {
	case "master_data":
		return "Stammdaten"
	case "care_schedule":
		return "Betreuungszeiten"
	case "offering":
		return "Angebote"
	case "excused_absence":
		return "Abwesenheit"
	case "enrollment":
		return "Anmeldung"
	default:
		return "Elternanfrage"
	}
}
