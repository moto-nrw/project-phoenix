/**
 * Die eine Navigationsliste des Träger-Portals ("moto träger", #5145).
 *
 * Seitenleiste und mobile Leiste rendern beide aus dieser Datei, ein neues
 * Ziel wird hier eingetragen und nicht an zwei Stellen. Es gibt zwei
 * Zusammenhänge an derselben Stelle (#5685): die Hauptnavigation, nach
 * Aufgaben gruppiert, und die Einstellungen mit dem Rückweg zur App. Analog zu
 * `school/shell/school-nav-items.ts`. Jedes Ziel nennt sein Konzept, nicht
 * sein Icon, und das Recht, das es öffnet; ohne Recht fehlt der Eintrag.
 */
import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";
import type { MotoConcept } from "~/lib/moto-concepts";
import { backConcept } from "~/lib/concepts/back";
import { billingConcept } from "~/lib/concepts/billing";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { careTimesConcept } from "~/lib/concepts/careTimes";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";
import { closingDaysConcept } from "~/lib/concepts/closingDays";
import { contributionsConcept } from "~/lib/concepts/contributions";
import { dashboardConcept } from "~/lib/concepts/dashboard";
import { documentsConcept } from "~/lib/concepts/documents";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { helpConcept } from "~/lib/concepts/help";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { parentsConcept } from "~/lib/concepts/parents";
import { peopleConcept } from "~/lib/concepts/people";
import { reportsConcept } from "~/lib/concepts/reports";
import { schoolsConcept } from "~/lib/concepts/schools";
import { settingsConcept } from "~/lib/concepts/settings";
import { staffConcept } from "~/lib/concepts/staff";
import { utilizationConcept } from "~/lib/concepts/utilization";

/** Das Recht, das „Mitglieder und Rollen" öffnet (ADR 0084, #4650). */
const ACCOUNTS_RIGHT = "traeger_accounts:manage";
/** Das Recht, das die Ferienzeiträume (#4659) und die Anmeldephasen (#4704) öffnet. */
const ENROLLMENT_RIGHT = "traeger_enrollment:manage";
/** Das Recht, das die Betreuungsverträge öffnet (#4678). */
const CONTRACTS_RIGHT = "traeger_contracts:manage";
/** Das Recht, das „Länger als gebucht“ ohne Namen öffnet (#4683). */
const REPORTS_RIGHT = "traeger_reports:read";
/** Das Recht, das die Abrechnungsläufe öffnet (#4668). */
const BILLING_RIGHT = "traeger_billing:manage";
/** Das Recht, das die Personalakten öffnet (Rolle Personal, #4705). */
const STAFF_RIGHT = "traeger_staff:manage";
/** Das Recht, das das Zugriffsprotokoll öffnet (#4655). */
const ACCESS_LOG_RIGHT = "traeger_access_log:read";
/**
 * Das Recht, das das Dashboard (#4674), die Stichtagszahlen (#4673), die Belegung (#4676), die allgemeinen Einstellungen (ADR 0086,
 * #4652), Briefkopf und Vorlagen (#4653, #4677) und die Abrechnungsvorgaben öffnet. Auf den
 * Abrechnungsvorgaben stehen die Bankverbindungen für alle mit diesem Recht, wie
 * zuvor auf dem Briefkopf; die Regeln und Rechnungsnummern nur mit dem Recht
 * „Abrechnung" (#5685). Jede Rolle mit „Abrechnung" hat auch dieses Recht.
 */
const OVERVIEW_RIGHT = "traeger_overview:read";

export interface TraegerNavItem {
  /** Stabiler Schlüssel für Listen und Tests. */
  readonly key: string;
  /** Pfad mit /traeger-Präfix; die Leisten schreiben ihn per `traegerPath` um. */
  readonly href: string;
  readonly label: string;
  /** Kurzer Name für die mobile Leiste, wo der aktive Name wenig Platz hat. */
  readonly shortLabel?: string;
  readonly concept: MotoConcept;
  /** Das Recht, ohne das der Eintrag fehlt; `null` für jede Person. */
  readonly right: string | null;
}

export interface TraegerNavGroup {
  readonly key: string;
  readonly label: string;
  readonly items: readonly TraegerNavItem[];
}

const TRAEGER_NAV_GROUPS: readonly TraegerNavGroup[] = [
  {
    key: "uebersicht",
    label: "Übersicht",
    items: [
      {
        // Die Zahlen von jetzt über alle Schulen (#4674).
        key: "dashboard",
        href: "/traeger/uebersicht",
        label: "Dashboard",
        concept: dashboardConcept,
        right: OVERVIEW_RIGHT,
      },
      {
        key: "schulen",
        href: "/traeger",
        label: "Schulen",
        concept: schoolsConcept,
        right: null,
      },
      {
        // Die festgehaltenen Zahlen je Monat, wie im Bericht des Betreibers (#4673).
        key: "stichtagszahlen",
        href: "/traeger/stichtagszahlen",
        label: "Stichtagszahlen",
        shortLabel: "Stichtage",
        concept: reportsConcept,
        right: OVERVIEW_RIGHT,
      },
      {
        // Die Kinder von heute je Schule, Gruppe und Betreuungsform (#4676).
        key: "belegung",
        href: "/traeger/belegung",
        label: "Belegung",
        concept: utilizationConcept,
        right: OVERVIEW_RIGHT,
      },
    ],
  },
  {
    key: "betreuung",
    label: "Betreuung",
    items: [
      {
        key: "ferien",
        href: "/traeger/ferien",
        label: "Ferienbetreuung",
        // The bottom bar shows the short word; the full name stays the
        // link's name.
        shortLabel: "Ferien",
        concept: holidayWeeksConcept,
        right: ENROLLMENT_RIGHT,
      },
      {
        // Anmeldephasen und Betreuungsangebote für mehrere Schulen (#4704).
        key: "anmeldephasen",
        href: "/traeger/anmeldephasen",
        label: "Anmeldephasen",
        concept: enrollmentsConcept,
        right: ENROLLMENT_RIGHT,
      },
      {
        // Schließtage des Trägers und Feiertage je Bundesland (#4702).
        key: "schliesstage",
        href: "/traeger/schliesstage",
        label: "Schließtage",
        concept: closingDaysConcept,
        right: OVERVIEW_RIGHT,
      },
      {
        // Betreuungsverträge aus angenommenen Anmeldungen (#4678).
        key: "vertraege",
        href: "/traeger/vertraege",
        label: "Verträge",
        concept: careContractsConcept,
        right: CONTRACTS_RIGHT,
      },
      {
        // Kinder, die länger da waren als gebucht, je Schule (#4683).
        key: "zeitvergleich",
        href: "/traeger/zeitvergleich",
        label: "Länger als gebucht",
        shortLabel: "Länger da",
        concept: careTimesConcept,
        right: REPORTS_RIGHT,
      },
      {
        // Vorhandene Dokumente für Eltern und Schulen freigeben (#4692).
        key: "dokumente",
        href: "/traeger/dokumente",
        label: "Dokumente",
        concept: documentsConcept,
        right: OVERVIEW_RIGHT,
      },
    ],
  },
  {
    // Personal über alle Schulen (ADR 0089): heute die Personalakten (#4705);
    // Verträge, Nachweise und Planung kommen hier hinzu.
    key: "personal",
    label: "Personal",
    items: [
      {
        key: "personalakten",
        href: "/traeger/personal",
        label: "Personalakten",
        shortLabel: "Personal",
        concept: staffConcept,
        right: STAFF_RIGHT,
      },
    ],
  },
  {
    // Drei Bereiche statt einer Seite mit zehn Reitern: die Läufe, die
    // Familien mit dem, was sie zahlen, und die Preise mit ihren
    // Ermäßigungen. Die Adressen bleiben; jeder Bereich zeigt nur seine Reiter.
    key: "finanzen",
    label: "Finanzen",
    items: [
      {
        key: "abrechnung",
        href: "/traeger/abrechnung",
        label: "Abrechnungsläufe",
        shortLabel: "Abrechnung",
        concept: billingConcept,
        right: BILLING_RIGHT,
      },
      {
        key: "familien",
        href: "/traeger/abrechnung/familien",
        label: "Familien und Zahlung",
        shortLabel: "Familien",
        concept: parentsConcept,
        right: BILLING_RIGHT,
      },
      {
        key: "preise",
        href: "/traeger/abrechnung/preise",
        label: "Preise und Ermäßigungen",
        shortLabel: "Preise",
        concept: contributionsConcept,
        right: BILLING_RIGHT,
      },
    ],
  },
];

/** Der Einstieg unten in der Hauptnavigation; sein Ziel ist der erste erlaubte Eintrag. */
export const TRAEGER_SETTINGS_NAV = {
  key: "einstellungen-bereich",
  label: "Einstellungen",
  concept: settingsConcept,
} as const satisfies Omit<TraegerNavItem, "href" | "right">;

/** Der oberste Eintrag der Einstellungen; das Ziel kommt aus `useTraegerReturnHref`. */
export const TRAEGER_BACK_NAV = {
  key: "zurueck-zur-app",
  label: "Zurück zur App",
  concept: backConcept,
} as const satisfies Omit<TraegerNavItem, "href" | "right">;

const TRAEGER_SETTINGS_NAV_ITEMS: readonly TraegerNavItem[] = [
  {
    key: "allgemein",
    href: "/traeger/einstellungen",
    label: "Allgemein",
    concept: settingsConcept,
    right: OVERVIEW_RIGHT,
  },
  {
    key: "mitglieder",
    href: "/traeger/einstellungen/mitglieder",
    label: "Mitglieder und Rollen",
    shortLabel: "Mitglieder",
    concept: peopleConcept,
    right: ACCOUNTS_RIGHT,
  },
  {
    key: "briefkopf",
    href: "/traeger/einstellungen/briefkopf",
    label: "Briefkopf und Vorlagen",
    shortLabel: "Briefkopf",
    concept: documentsConcept,
    right: OVERVIEW_RIGHT,
  },
  {
    key: "abrechnungsvorgaben",
    href: "/traeger/einstellungen/abrechnung",
    label: "Abrechnungsvorgaben",
    shortLabel: "Vorgaben",
    concept: contributionsConcept,
    right: OVERVIEW_RIGHT,
  },
  {
    key: "dokumentarten",
    href: "/traeger/einstellungen/dokumentarten",
    label: "Dokumentarten",
    shortLabel: "Dokumentart",
    concept: billingConcept,
    right: BILLING_RIGHT,
  },
  {
    key: "zugriffsprotokoll",
    href: "/traeger/einstellungen/zugriffsprotokoll",
    label: "Zugriffsprotokoll",
    shortLabel: "Protokoll",
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

/** Die Gruppen der Hauptnavigation; eine Gruppe ohne erlaubtes Ziel fehlt. */
export function traegerNavGroupsFor(
  session: Session | null,
): readonly TraegerNavGroup[] {
  return TRAEGER_NAV_GROUPS.map((group) => ({
    ...group,
    items: visibleFor(group.items, session),
  })).filter((group) => group.items.length > 0);
}

/**
 * Die Einträge der Einstellungen, die diese Person sehen darf. Eine leere
 * Liste heißt: kein Eintrag „Einstellungen".
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
