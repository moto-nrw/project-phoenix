import type { AuthTestimonialPanelCopy } from "./auth-shell";

const parentAuthTestimonialKeys = [
  "infoHub",
  "easyReplies",
  "lookUpLater",
  "organized",
] as const;

type ParentAuthTestimonialKey = (typeof parentAuthTestimonialKeys)[number];
type ParentAuthTestimonialField = "quote" | "author" | "role" | "metric";

type ParentAuthShellMessageKey =
  | "badges.audience"
  | "badges.hosting"
  | "testimonialButtonLabel"
  | `testimonials.${ParentAuthTestimonialKey}.${ParentAuthTestimonialField}`;

// The shell stores `testimonialButtonLabel` as a raw "...{number}..." template
// and fills the index itself via String.replace (see auth-shell.tsx). We must
// therefore read that message *unformatted* — calling the translator normally
// makes next-intl try to resolve the {number} ICU argument and throw
// FORMATTING_ERROR. `raw` returns the literal template the shell expects.
type ParentAuthShellTranslate = ((key: ParentAuthShellMessageKey) => string) & {
  raw: (key: ParentAuthShellMessageKey) => string;
};

export function buildParentAuthShellCopy(
  t: ParentAuthShellTranslate,
): AuthTestimonialPanelCopy {
  return {
    badges: {
      audience: t("badges.audience"),
      hosting: t("badges.hosting"),
    },
    testimonialButtonLabel: t.raw("testimonialButtonLabel"),
    testimonials: parentAuthTestimonialKeys.map((key) => ({
      quote: t(`testimonials.${key}.quote`),
      author: t(`testimonials.${key}.author`),
      role: t(`testimonials.${key}.role`),
      metric: t(`testimonials.${key}.metric`),
    })),
  };
}
