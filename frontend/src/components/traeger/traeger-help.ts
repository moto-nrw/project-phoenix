/**
 * The help entry of the Träger portal (#4649): the article for the page the
 * Träger-Büro is on. The shared header's `?` (#5145, `ContextHelpLink`)
 * builds the link with the Träger role, so the help opens the right sidebar,
 * and with the page as `return_to`.
 */
import { HELP_TOPICS, type HelpTopicId } from "~/lib/help-topics";

/** `/ferien/{id}/anmeldungen` and its registrations (#4663). */
const HOLIDAY_REGISTRATIONS_PAGE = /^\/ferien\/[^/]+\/anmeldungen(?:\/|$)/;

/** `/ferien/{id}/formular`, the form of a holiday period (#5198). */
const HOLIDAY_FORM_PAGE = /^\/ferien\/[^/]+\/formular$/;

function onPage(page: string, path: string): boolean {
  return page === path || page.startsWith(`${path}/`);
}

/**
 * The help article for a Träger page (pathname with or without /traeger).
 * Pages without an article of their own lead to the sign-in article, which
 * explains the portal.
 */
export function getTraegerHelpTopicForPath(pathname: string): HelpTopicId {
  const page = pathname.replace(/^\/traeger(?=\/|$)/, "") || "/";
  if (onPage(page, "/einstellungen")) return HELP_TOPICS.traegerSettings;
  if (onPage(page, "/briefkopf")) return HELP_TOPICS.traegerBriefkopf;
  if (onPage(page, "/beitraege")) return HELP_TOPICS.traegerHolidayContribution;
  if (onPage(page, "/abrechnung")) return HELP_TOPICS.traegerBillingRun;
  if (HOLIDAY_FORM_PAGE.test(page)) return HELP_TOPICS.traegerHolidayForm;
  if (HOLIDAY_REGISTRATIONS_PAGE.test(page)) {
    return HELP_TOPICS.traegerHolidayRegistrationReview;
  }
  return HELP_TOPICS.traegerLogin;
}
