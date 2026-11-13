import type { DecisionStatus } from "~/lib/enrollment-admin-api";

/**
 * What a decision button does for one child (#6749), shared by the request
 * detail and the quick actions of the phase list. Approved, waitlisted and
 * rejected always reach the parents by e-mail, either at once or as one
 * collected e-mail after the last child of the request; „Zur Prüfung“ sends
 * nothing.
 */
export const DECISION_HINTS: Readonly<Record<DecisionStatus, string>> = {
  approved:
    "Vergibt den Platz verbindlich und informiert die Eltern per E-Mail.",
  waitlisted:
    "Setzt das Kind auf die Warteliste und informiert die Eltern per E-Mail.",
  rejected:
    "Lehnt die Anmeldung des Kindes ab und informiert die Eltern per E-Mail.",
  under_review:
    "Setzt das Kind auf „In Prüfung“, ohne die Eltern zu informieren.",
};
