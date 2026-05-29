package enrollment

import (
	"context"
	"encoding/json"
	"strings"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

const schoolLoginImageKey = "loginImageUrl"
const loginImageUploadPrefix = "/uploads/login-images/"

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

func motoLogoURL(baseURL string) string {
	return absoluteBrandURL(baseURL, "/images/moto_transparent.png")
}

func schoolEmailLogoURL(baseURL string, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, loginImageUploadPrefix) {
		filename := strings.TrimPrefix(rawURL, loginImageUploadPrefix)
		if filename == "" || strings.Contains(filename, "/") {
			return ""
		}
		return absoluteBrandURL(baseURL, "/api/public/login-image/"+filename)
	}
	return absoluteBrandURL(baseURL, rawURL)
}

func absoluteBrandURL(baseURL string, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return baseURL + rawURL
	}
	return baseURL + "/" + rawURL
}
