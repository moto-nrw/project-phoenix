/**
 * Die eine Navigationsliste der Eltern-App.
 *
 * Bottom-Navigation, Seitennavigation und das "Mehr"-Menue rendern alle aus
 * dieser Datei. Ein neues Ziel wird hier eingetragen, nicht an drei Stellen.
 *
 * Reihenfolge nach Entscheidung E8 der Spezifikation: Start, Kinder,
 * Nachrichten, Kalender, Mehr.
 *
 * Ein Eintrag nennt sein Konzept, nicht sein Icon. Glyph, Ton und Gewicht
 * kommen aus der Konzeptdefinition (`~/lib/concepts`), damit die Eltern-App dieselbe Bildsprache
 * spricht wie das uebrige Produkt.
 */
import type { MotoConcept } from "~/lib/moto-concepts";
import { calendarConcept } from "~/lib/concepts/calendar";
import { childrenConcept } from "~/lib/concepts/children";
import { dashboardConcept } from "~/lib/concepts/dashboard";
import { documentsConcept } from "~/lib/concepts/documents";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { helpConcept } from "~/lib/concepts/help";
import { logoutConcept } from "~/lib/concepts/logout";
import { mealPlanConcept } from "~/lib/concepts/mealPlan";
import { moreConcept } from "~/lib/concepts/more";
import { newsConcept } from "~/lib/concepts/news";
import { parentConversationsConcept } from "~/lib/concepts/parentConversations";
import { settingsConcept } from "~/lib/concepts/settings";

/**
 * Woher ein Zaehler kommt. Die Huelle loest das zu einer Zahl auf, damit diese
 * Datei frei von Datenzugriff bleibt.
 */
export type ParentNavBadgeSource = "messages" | "news";

/**
 * Ob ein Ziel ueberhaupt angeboten wird. Elternbriefe und Essensplan gibt es
 * nur, wenn mindestens eine verknuepfte Schule sie fuehrt.
 */
export type ParentNavGate = "news" | "mealPlan";

export interface ParentNavItem {
  /** Stabiler Schluessel fuer Listen und Tests. */
  readonly key: string;
  readonly href: string;
  /**
   * Ziel ausserhalb des Portals. Oeffnet in einem neuen Tab und bekommt nie
   * einen Aktivzustand -- die Adresse taucht in diesem Portal nicht auf.
   */
  readonly external?: boolean;
  /** Schluessel im Katalog "parentNav", identisch in allen Sprachkatalogen. */
  readonly tKey: string;
  readonly concept: MotoConcept;
  readonly badge?: ParentNavBadgeSource;
  readonly gate?: ParentNavGate;
}

/** Ein Eintrag hinter "Mehr" ist entweder ein Ziel oder eine Handlung. */
export type ParentMoreItem =
  | ({ readonly kind: "link" } & ParentNavItem)
  | {
      readonly kind: "action";
      readonly key: string;
      readonly action: "logout";
      readonly tKey: string;
      readonly concept: MotoConcept;
    };

/** Die vier Alltagsziele. Das fuenfte Feld der Bottom-Navigation ist "Mehr". */
export const PARENT_PRIMARY_NAV: readonly ParentNavItem[] = [
  {
    key: "start",
    href: "/parents",
    tKey: "start",
    concept: dashboardConcept,
  },
  {
    key: "children",
    href: "/parents/children",
    tKey: "children",
    concept: childrenConcept,
  },
  {
    key: "messages",
    href: "/parents/messages",
    tKey: "messages",
    concept: parentConversationsConcept,
    badge: "messages",
  },
  {
    key: "calendar",
    href: "/parents/calendar",
    tKey: "calendar",
    concept: calendarConcept,
  },
];

/**
 * Hinter "Mehr". Alles, was nicht taeglich gebraucht wird.
 *
 * Der Offen-Zaehler der Elternbriefe wird auf das "Mehr"-Symbol aufaddiert.
 * Sonst bliebe eine ausstehende Aufgabe unsichtbar, weil ihr
 * Eintrag im Sheet liegt und die Bottom-Navigation ihn nicht zeigt.
 */
export const PARENT_MORE_NAV: readonly ParentMoreItem[] = [
  {
    kind: "link",
    key: "news",
    href: "/parents/news",
    tKey: "news",
    concept: newsConcept,
    badge: "news",
    gate: "news",
  },
  {
    kind: "link",
    key: "mealPlan",
    href: "/parents/meal-plan",
    tKey: "mealPlan",
    concept: mealPlanConcept,
    gate: "mealPlan",
  },
  {
    // Dokumente des Traegers (#4671), etwa Bestaetigungsschreiben der Ferien.
    kind: "link",
    key: "documents",
    href: "/parents/documents",
    tKey: "documents",
    concept: documentsConcept,
  },
  {
    // Die Anleitung liegt ausserhalb des Portals, auf der oeffentlichen
    // /help-Strecke. `role=parent` ueberspringt ihre Eingangsfrage: wer
    // hier klickt, ist ein Elternteil. Die drei OGS-Einstellungen aendern in
    // der Eltern-Anleitung keinen Ablauf und bleiben deshalb weg.
    kind: "link",
    key: "help",
    href: "/help?role=parent",
    tKey: "help",
    concept: helpConcept,
    external: true,
  },
  {
    kind: "link",
    key: "settings",
    href: "/parents/settings",
    tKey: "settings",
    concept: settingsConcept,
  },
  {
    kind: "link",
    key: "enroll",
    href: "/parents/anmeldung",
    tKey: "enroll",
    concept: enrollmentsConcept,
  },
  {
    kind: "action",
    key: "logout",
    action: "logout",
    tKey: "logout",
    concept: logoutConcept,
  },
];

/** Das fuenfte Feld der Bottom-Navigation. Kein eigener href. */
export const PARENT_MORE_ENTRY = {
  key: "more",
  tKey: "more",
  concept: moreConcept,
} as const satisfies { key: string; tKey: string; concept: MotoConcept };

/** Zaehler, die auf das "Mehr"-Symbol aufaddiert werden. */
export const PARENT_MORE_BADGE_SOURCES: readonly ParentNavBadgeSource[] = [
  "news",
];
