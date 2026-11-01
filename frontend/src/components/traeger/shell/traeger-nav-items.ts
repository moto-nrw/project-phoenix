/**
 * Die eine Navigationsliste des Träger-Portals ("moto träger", #5145).
 *
 * Seitenleiste und mobile Leiste rendern beide aus dieser Datei, ein neues
 * Ziel wird hier eingetragen und nicht an zwei Stellen. Die Seitenleiste
 * zeigt die Arbeitsbereiche oben und die Einstellungen unten als
 * einklappbaren Abschnitt; die mobile Leiste reiht beide hintereinander. Analog zu
 * `school/shell/school-nav-items.ts`. Jedes Ziel nennt sein Konzept, nicht
 * sein Icon, und das Recht, das es öffnet; ohne Recht fehlt der Eintrag.
 */
import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";
import type { MotoConcept } from "~/lib/moto-concepts";
import { billingConcept } from "~/lib/concepts/billing";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";
import { contributionsConcept } from "~/lib/concepts/contributions";
import { documentsConcept } from "~/lib/concepts/documents";
import { helpConcept } from "~/lib/concepts/help";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { peopleConcept } from "~/lib/concepts/people";
import { schoolsConcept } from "~/lib/concepts/schools";
import { settingsConcept } from "~/lib/concepts/settings";

/** Das Recht, das die Seite „Personen" öffnet (ADR 0084, #4650). */
const ACCOUNTS_RIGHT = "traeger_accounts:manage";
/** Das Recht, das die Ferienzeiträume öffnet (#4659). */
const ENROLLMENT_RIGHT = "traeger_enrollment:manage";
/** Das Recht für die Beitragsregeln (#4664) und die Abrechnungsläufe (#4668). */
const BILLING_RIGHT = "traeger_billing:manage";
/** Das Recht, das das Zugriffsprotokoll öffnet (#4655). */
const ACCESS_LOG_RIGHT = "traeger_access_log:read";
/**
 * Das Recht, das die Einstellungen (ADR 0086, #4652) und den Briefkopf
 * (#4653) öffnet.
 */
const OVERVIEW_RIGHT = "traeger_overview:read";

export interface TraegerNavItem {
  /** Stabiler Schlüssel für Listen und Tests. */
  readonly key: string;
  /** Pfad mit /traeger-Präfix; die Leisten schreiben ihn per `traegerPath` um. */
  readonly href: string;
  readonly label: string;
  readonly concept: MotoConcept;
  /** Das Recht, ohne das der Eintrag fehlt; `null` für jede Person. */
  readonly right: string | null;
}

const TRAEGER_PRIMARY_NAV: readonly TraegerNavItem[] = [
  {
    key: "schulen",
    href: "/traeger",
    label: "Schulen",
    concept: schoolsConcept,
    right: null,
  },
  {
    key: "ferien",
    href: "/traeger/ferien",
    label: "Ferien",
    concept: holidayWeeksConcept,
    right: ENROLLMENT_RIGHT,
  },
  {
    key: "beitraege",
    href: "/traeger/beitraege",
    label: "Beiträge",
    concept: contributionsConcept,
    right: BILLING_RIGHT,
  },
  {
    key: "abrechnung",
    href: "/traeger/abrechnung",
    label: "Abrechnung",
    concept: billingConcept,
    right: BILLING_RIGHT,
  },
  {
    key: "personen",
    href: "/traeger/accounts",
    label: "Personen",
    concept: peopleConcept,
    right: ACCOUNTS_RIGHT,
  },
];

/**
 * Unten angeheftet und einklappbar wie im Schul-Portal (#5242): die
 * Einstellungen des Trägers. Selten gebraucht, deshalb nicht zwischen den
 * Arbeitsbereichen (#5241). Jeder Unterpunkt hat sein eigenes Recht.
 */
export const TRAEGER_SETTINGS_NAV = {
  key: "einstellungen-bereich",
  label: "Einstellungen",
  concept: settingsConcept,
} as const satisfies Omit<TraegerNavItem, "href" | "right">;

const TRAEGER_SETTINGS_NAV_ITEMS: readonly TraegerNavItem[] = [
  {
    key: "briefkopf",
    href: "/traeger/briefkopf",
    label: "Briefkopf",
    concept: documentsConcept,
    right: OVERVIEW_RIGHT,
  },
  {
    key: "einstellungen",
    href: "/traeger/einstellungen",
    label: "Einstellungen",
    concept: settingsConcept,
    right: OVERVIEW_RIGHT,
  },
  {
    key: "zugriffsprotokoll",
    href: "/traeger/access-log",
    label: "Zugriffsprotokoll",
    concept: changeHistoryConcept,
    right: ACCESS_LOG_RIGHT,
  },
];

/**
 * Unten angeheftet: die Hilfe. Sie liegt außerhalb des Portals (öffentliche
 * /help-Strecke) und öffnet deshalb in einem neuen Tab, mit der Rolle, damit
 * sie die Eingangsfrage überspringt.
 */
export const TRAEGER_HELP_NAV = {
  key: "help",
  href: "/help?role=traeger",
  label: "Hilfe",
  concept: helpConcept,
} as const satisfies Omit<TraegerNavItem, "right">;

/** Die Arbeitsbereiche, die diese Person sehen darf, in der Reihenfolge der Liste. */
export function traegerNavItemsFor(
  session: Session | null,
): readonly TraegerNavItem[] {
  return visibleFor(TRAEGER_PRIMARY_NAV, session);
}

/**
 * Die Unterpunkte der Einstellungen, die diese Person sehen darf. Eine leere
 * Liste heißt: kein Abschnitt „Einstellungen".
 */
export function traegerSettingsNavItemsFor(
  session: Session | null,
): readonly TraegerNavItem[] {
  return visibleFor(TRAEGER_SETTINGS_NAV_ITEMS, session);
}

function visibleFor(
  items: readonly TraegerNavItem[],
  session: Session | null,
): readonly TraegerNavItem[] {
  return items.filter(
    (item) => item.right === null || hasPermission(session, item.right),
  );
}
