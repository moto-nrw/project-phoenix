package organizationtenancy

import "strings"

// maxDemoSlugName keeps the slug, which every account email and username of
// the school carries, well inside a DNS label.
const maxDemoSlugName = 30

var demoSlugReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")

// DemoSchoolSlug derives the slug of a demo school from the name a prospect
// typed: lowercase ASCII letters, digits and single hyphens, then the suffix
// that makes it unique. A name without a usable character yields "ogs".
func DemoSchoolSlug(schoolName, suffix string) string {
	var slug strings.Builder
	for _, r := range demoSlugReplacer.Replace(strings.ToLower(schoolName)) {
		if slug.Len() >= maxDemoSlugName {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			slug.WriteRune(r)
		} else if slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-") {
			slug.WriteByte('-')
		}
	}
	name := strings.Trim(slug.String(), "-")
	if name == "" {
		name = "ogs"
	}
	return name + "-" + suffix
}
