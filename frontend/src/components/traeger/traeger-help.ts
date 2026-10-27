/**
 * The help entry of the Träger portal (#4649): the article for the page the
 * Träger-Büro is on, with the Träger role so the help opens the right
 * sidebar. The help is host-agnostic, so the article path is not rewritten;
 * `return_to` is the page's path on the Träger host.
 */
import { HELP_TOPICS } from "~/lib/help-topics";

function traegerHelpLink(topic: string, returnTo: string): string {
  return `/help/${topic}?${new URLSearchParams({
    role: "traeger",
    return_to: returnTo,
  }).toString()}`;
}

const TRAEGER_HELP_HREF = traegerHelpLink(HELP_TOPICS.traegerLogin, "/");

const TRAEGER_SETTINGS_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerSettings,
  "/einstellungen",
);

const TRAEGER_BRIEFKOPF_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerBriefkopf,
  "/briefkopf",
);

const TRAEGER_CONTRIBUTION_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerHolidayContribution,
  "/beitraege",
);

const TRAEGER_BILLING_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerBillingRun,
  "/abrechnung",
);

/** `/ferien/{id}/anmeldungen` and its registrations (#4663). */
const HOLIDAY_REGISTRATIONS_PAGE = /^\/ferien\/[^/]+\/anmeldungen(?:\/|$)/;

function onPage(page: string, path: string): boolean {
  return page === path || page.startsWith(`${path}/`);
}

/** The help article for a Träger page (pathname with or without /traeger). */
export function traegerHelpHref(pathname: string): string {
  const page = pathname.replace(/^\/traeger/, "") || "/";
  if (onPage(page, "/einstellungen")) return TRAEGER_SETTINGS_HELP_HREF;
  if (onPage(page, "/briefkopf")) return TRAEGER_BRIEFKOPF_HELP_HREF;
  if (onPage(page, "/beitraege")) return TRAEGER_CONTRIBUTION_HELP_HREF;
  if (onPage(page, "/abrechnung")) return TRAEGER_BILLING_HELP_HREF;
  if (HOLIDAY_REGISTRATIONS_PAGE.test(page)) {
    return traegerHelpLink(HELP_TOPICS.traegerHolidayRegistrationReview, page);
  }
  return TRAEGER_HELP_HREF;
}
