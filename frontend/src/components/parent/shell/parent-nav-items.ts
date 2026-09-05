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
 * kommen aus `MOTO_CONCEPTS`, damit die Eltern-App dieselbe Bildsprache
 * spricht wie das uebrige Produkt.
 */
import type { MotoConceptKey } from "~/lib/moto-concepts";

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
  /** Schluessel im Katalog "parentNav", identisch in de, en, ru und sq. */
  readonly tKey: string;
  readonly concept: MotoConceptKey;
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
      readonly concept: MotoConceptKey;
    };

/** Die vier Alltagsziele. Das fuenfte Feld der Bottom-Navigation ist "Mehr". */
export const PARENT_PRIMARY_NAV: readonly ParentNavItem[] = [
  {
    key: "start",
    href: "/parents",
    tKey: "start",
    concept: "dashboard",
  },
  {
    key: "children",
    href: "/parents/children",
    tKey: "children",
    concept: "children",
  },
  {
    key: "messages",
    href: "/parents/messages",
    tKey: "messages",
    concept: "parentConversations",
    badge: "messages",
  },
  {
    key: "calendar",
    href: "/parents/calendar",
    tKey: "calendar",
    concept: "calendar",
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
    concept: "news",
    badge: "news",
    gate: "news",
  },
  {
    kind: "link",
    key: "mealPlan",
    href: "/parents/meal-plan",
    tKey: "mealPlan",
    concept: "mealPlan",
    gate: "mealPlan",
  },
  {
    kind: "link",
    key: "settings",
    href: "/parents/settings",
    tKey: "settings",
    concept: "settings",
  },
  {
    kind: "link",
    key: "enroll",
    href: "/parents/anmeldung",
    tKey: "enroll",
    concept: "enrollments",
  },
  {
    kind: "action",
    key: "logout",
    action: "logout",
    tKey: "logout",
    concept: "logout",
  },
];

/** Das fuenfte Feld der Bottom-Navigation. Kein eigener href. */
export const PARENT_MORE_ENTRY = {
  key: "more",
  tKey: "more",
  concept: "more",
} as const satisfies { key: string; tKey: string; concept: MotoConceptKey };

/** Zaehler, die auf das "Mehr"-Symbol aufaddiert werden. */
export const PARENT_MORE_BADGE_SOURCES: readonly ParentNavBadgeSource[] = [
  "news",
];
