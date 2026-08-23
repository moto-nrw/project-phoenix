package enrollment

import (
	"context"
	"encoding/json"
	"strings"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
)

const schoolLoginImageKey = "loginImageUrl"

func emailBrandForSchool(ctx context.Context, schoolRepo platformModels.SchoolRepository, tenantID int64, baseURL string) (string, string) {
	if schoolRepo == nil || tenantID == 0 {
		return "", ""
	}
	school, err := schoolRepo.FindByID(ctx, tenantID)
	if err != nil || school == nil || school.IsDeleted() {
		return "", ""
	}
	return school.Name, schoolEmailLogoURL(baseURL, schoolLoginImageURL(school.Settings))
}

func schoolLoginImageURL(rawSettings string) string {
	if strings.TrimSpace(rawSettings) == "" {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(rawSettings), &settings); err != nil {
		return ""
	}
	url, _ := settings[schoolLoginImageKey].(string)
	return strings.TrimSpace(url)
}

// The three helpers below delegate to the shared emailbranding package so the
// enrollment and parent-announcement mails resolve logo URLs identically. They
// stay as package-local wrappers to keep existing call sites and tests stable.
func motoLogoURL(baseURL string) string {
	return emailbranding.MotoLogoURL(baseURL)
}

func schoolEmailLogoURL(baseURL string, rawURL string) string {
	return emailbranding.SchoolLogoURL(baseURL, rawURL)
}
