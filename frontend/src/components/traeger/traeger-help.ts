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
  if (onPage(page, "/uebersicht")) return HELP_TOPICS.traegerDashboard;
  if (onPage(page, "/stichtagszahlen")) {
    return HELP_TOPICS.traegerKeyDateCounts;
  }
  if (onPage(page, "/belegung")) return HELP_TOPICS.traegerOccupancy;
  if (onPage(page, "/schulen")) return HELP_TOPICS.traegerSchoolView;
  if (onPage(page, "/dokumente")) return HELP_TOPICS.traegerSharedDocuments;
  if (onPage(page, "/einstellungen/mitglieder")) {
    return HELP_TOPICS.traegerAccounts;
  }
  if (onPage(page, "/einstellungen/zugriffsprotokoll")) {
    return HELP_TOPICS.traegerAccessLog;
  }
  if (onPage(page, "/einstellungen/briefkopf/vorlagen")) {
    return HELP_TOPICS.traegerDocumentTemplates;
  }
  if (onPage(page, "/einstellungen/briefkopf")) {
    return HELP_TOPICS.traegerBriefkopf;
  }
  if (onPage(page, "/einstellungen/abrechnung")) {
    return HELP_TOPICS.traegerHolidayContribution;
  }
  if (onPage(page, "/einstellungen")) return HELP_TOPICS.traegerSettings;
  if (
    onPage(page, "/abrechnung/monatsbeitraege") ||
    onPage(page, "/abrechnung/preise")
  ) {
    return HELP_TOPICS.traegerCareMonths;
  }
  if (onPage(page, "/abrechnung/mensa")) return HELP_TOPICS.traegerCareMeals;
  if (
    onPage(page, "/abrechnung/ermaessigungen") ||
    onPage(page, "/abrechnung/nachweise") ||
    onPage(page, "/abrechnung/kinderzahl")
  ) {
    return HELP_TOPICS.traegerCareReductions;
  }
  if (onPage(page, "/abrechnung/lastschrift")) {
    return HELP_TOPICS.traegerSepaMandates;
  }
  if (onPage(page, "/abrechnung/kostentraeger")) {
    return HELP_TOPICS.traegerCareCostShares;
  }
  if (onPage(page, "/abrechnung")) return HELP_TOPICS.traegerBillingRun;
  if (onPage(page, "/vertraege")) return HELP_TOPICS.traegerCareContracts;
  if (onPage(page, "/zeitvergleich"))
    return HELP_TOPICS.traegerCareTimeOverruns;
  if (HOLIDAY_FORM_PAGE.test(page)) return HELP_TOPICS.traegerHolidayForm;
  if (HOLIDAY_REGISTRATIONS_PAGE.test(page)) {
    return HELP_TOPICS.traegerHolidayRegistrationReview;
  }
  return HELP_TOPICS.traegerLogin;
}
