// Package openholidays reads the German school vacations from the
// OpenHolidays API (https://openholidaysapi.org, #5345). One request covers
// every federal state; the API answers at most 1095 days per request.
package openholidays

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BaseURL is the canonical OpenHolidays API.
const BaseURL = "https://openholidaysapi.org"

// MaxRangeDays is the longest window the API answers in one request.
const MaxRangeDays = 1095

// requestTimeout bounds one request when the caller passes no client.
const requestTimeout = 10 * time.Second

// maxResponseBytes bounds the body read; two years of all states are about
// 60 KB.
const maxResponseBytes = 4 << 20

// Vacation is one school vacation of one federal state. Region is the ISO
// code ("DE-NW"); StartDate and EndDate are YYYY-MM-DD, both inclusive.
type Vacation = struct {
	Region    string
	Name      string
	StartDate string
	EndDate   string
}

// Client asks the SchoolHolidays endpoint.
type Client struct {
	httpClient *http.Client
	baseURL    string
	states     []string
}

// New builds a client for the federal states. A nil httpClient falls back
// to a client with a 10-second timeout; an empty baseURL falls back to
// BaseURL. A vacation the API marks as nationwide is returned once per state.
func New(httpClient *http.Client, baseURL string, states []string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}
	if baseURL == "" {
		baseURL = BaseURL
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/"), states: states}
}

type apiName struct {
	Language string `json:"language"`
	Text     string `json:"text"`
}

type apiSubdivision struct {
	Code string `json:"code"`
}

type apiHoliday struct {
	StartDate    string           `json:"startDate"`
	EndDate      string           `json:"endDate"`
	Type         string           `json:"type"`
	Name         []apiName        `json:"name"`
	Nationwide   bool             `json:"nationwide"`
	Subdivisions []apiSubdivision `json:"subdivisions"`
}

// SchoolVacations returns the school vacations of every German federal state
// touching [from, to]. Any transport, status or decoding problem is an
// error; the caller keeps what it stored before.
func (c *Client) SchoolVacations(ctx context.Context, from, to string) ([]Vacation, error) {
	query := url.Values{}
	query.Set("countryIsoCode", "DE")
	query.Set("languageIsoCode", "DE")
	query.Set("validFrom", from)
	query.Set("validTo", to)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/SchoolHolidays?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("school vacations: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("school vacations: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("school vacations: unexpected status %d", resp.StatusCode)
	}
	var holidays []apiHoliday
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&holidays); err != nil {
		return nil, fmt.Errorf("school vacations: decode response: %w", err)
	}
	return vacations(holidays, c.states)
}

func vacations(holidays []apiHoliday, states []string) ([]Vacation, error) {
	result := make([]Vacation, 0, len(holidays))
	for _, holiday := range holidays {
		if holiday.Type != "" && holiday.Type != "School" {
			continue
		}
		start, startErr := time.Parse(time.DateOnly, holiday.StartDate)
		end, endErr := time.Parse(time.DateOnly, holiday.EndDate)
		if startErr != nil || endErr != nil || end.Before(start) {
			return nil, fmt.Errorf("school vacations: invalid range %q to %q", holiday.StartDate, holiday.EndDate)
		}
		name := germanName(holiday.Name)
		if name == "" {
			return nil, fmt.Errorf("school vacations: vacation from %s has no name", holiday.StartDate)
		}
		for _, region := range regions(holiday, states) {
			result = append(result, Vacation{Region: region, Name: name, StartDate: holiday.StartDate, EndDate: holiday.EndDate})
		}
	}
	return result, nil
}

func regions(holiday apiHoliday, states []string) []string {
	if holiday.Nationwide {
		return states
	}
	codes := make([]string, 0, len(holiday.Subdivisions))
	for _, subdivision := range holiday.Subdivisions {
		codes = append(codes, subdivision.Code)
	}
	return codes
}

func germanName(names []apiName) string {
	for _, name := range names {
		if strings.EqualFold(name.Language, "DE") {
			return strings.TrimSpace(name.Text)
		}
	}
	if len(names) > 0 {
		return strings.TrimSpace(names[0].Text)
	}
	return ""
}
