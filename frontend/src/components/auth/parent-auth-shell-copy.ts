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

type ParentAuthShellTranslate = (key: ParentAuthShellMessageKey) => string;

export function buildParentAuthShellCopy(
  t: ParentAuthShellTranslate,
): AuthTestimonialPanelCopy {
  return {
    badges: {
      audience: t("badges.audience"),
      hosting: t("badges.hosting"),
    },
    testimonialButtonLabel: t("testimonialButtonLabel"),
    testimonials: parentAuthTestimonialKeys.map((key) => ({
      quote: t(`testimonials.${key}.quote`),
      author: t(`testimonials.${key}.author`),
      role: t(`testimonials.${key}.role`),
      metric: t(`testimonials.${key}.metric`),
    })),
  };
}
