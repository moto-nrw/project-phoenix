/**
 * Die eine Navigationsliste des Träger-Portals ("moto träger", #5145).
 *
 * Seitenleiste und mobile Leiste rendern beide aus dieser Datei, ein neues
 * Ziel wird hier eingetragen und nicht an zwei Stellen. Analog zu
 * `school/shell/school-nav-items.ts`. Jedes Ziel nennt sein Konzept, nicht
 * sein Icon, und das Recht, das es öffnet; ohne Recht fehlt der Eintrag.
 */
import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";
import type { MotoConceptKey } from "~/lib/moto-concepts";

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
  readonly concept: MotoConceptKey;
  /** Das Recht, ohne das der Eintrag fehlt; `null` für jede Person. */
  readonly right: string | null;
}

const TRAEGER_PRIMARY_NAV: readonly TraegerNavItem[] = [
  {
    key: "schulen",
    href: "/traeger",
    label: "Schulen",
    concept: "schools",
    right: null,
  },
  {
    key: "ferien",
    href: "/traeger/ferien",
    label: "Ferien",
    concept: "holidayWeeks",
    right: ENROLLMENT_RIGHT,
  },
  {
    key: "beitraege",
    href: "/traeger/beitraege",
    label: "Beiträge",
    concept: "contributions",
    right: BILLING_RIGHT,
  },
  {
    key: "abrechnung",
    href: "/traeger/abrechnung",
    label: "Abrechnung",
    concept: "billing",
    right: BILLING_RIGHT,
  },
  {
    key: "personen",
    href: "/traeger/accounts",
    label: "Personen",
    concept: "people",
    right: ACCOUNTS_RIGHT,
  },
  {
    key: "zugriffsprotokoll",
    href: "/traeger/access-log",
    label: "Zugriffsprotokoll",
    concept: "changeHistory",
    right: ACCESS_LOG_RIGHT,
  },
  {
    key: "briefkopf",
    href: "/traeger/briefkopf",
    label: "Briefkopf",
    concept: "documents",
    right: OVERVIEW_RIGHT,
  },
  {
    key: "einstellungen",
    href: "/traeger/einstellungen",
    label: "Einstellungen",
    concept: "settings",
    right: OVERVIEW_RIGHT,
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
  concept: "help",
} as const satisfies Omit<TraegerNavItem, "right">;

/** Die Ziele, die diese Person sehen darf, in der Reihenfolge der Liste. */
export function traegerNavItemsFor(
  session: Session | null,
): readonly TraegerNavItem[] {
  return TRAEGER_PRIMARY_NAV.filter(
    (item) => item.right === null || hasPermission(session, item.right),
  );
}
