/**
 * Die eine Navigationsliste der Eltern-App.
 *
 * Bottom-Navigation, Seitennavigation und das "Mehr"-Menue rendern alle aus
 * dieser Datei. Ein neues Ziel wird hier eingetragen, nicht an drei Stellen.
 *
 * Reihenfolge nach Entscheidung E8 der Spezifikation: Start, Kinder,
 * Nachrichten, Kalender, Mehr.
 */
import {
  Bell,
  CalendarBlank,
  ChatCircleText,
  DotsThree,
  ForkKnife,
  House,
  Megaphone,
  SignOut,
  Translate,
  UserPlus,
  Users,
  type Icon,
  type IconWeight,
} from "./parent-icons";

/** Standardgewicht der Eltern-App. "fill" markiert den aktiven Zustand. */
export const PARENT_ICON_WEIGHT: IconWeight = "regular";
export const PARENT_ICON_WEIGHT_ACTIVE: IconWeight = "fill";

/**
 * Woher ein Zaehler kommt. Die Huelle loest das zu einer Zahl auf, damit diese
 * Datei frei von Datenzugriff bleibt.
 */
export type ParentNavBadgeSource = "messages" | "news";

/**
 * Ob ein Ziel ueberhaupt angeboten wird. Neuigkeiten und Essensplan gibt es
 * nur, wenn mindestens eine verknuepfte Schule sie fuehrt.
 */
export type ParentNavGate = "news" | "mealPlan";

export interface ParentNavItem {
  /** Stabiler Schluessel fuer Listen und Tests. */
  readonly key: string;
  readonly href: string;
  /** Schluessel im Katalog "parentNav", identisch in de, en, ru und sq. */
  readonly tKey: string;
  readonly icon: Icon;
  readonly badge?: ParentNavBadgeSource;
  readonly gate?: ParentNavGate;
}

/** Ein Eintrag hinter "Mehr" ist entweder ein Ziel oder eine Handlung. */
export type ParentMoreItem =
  | ({ readonly kind: "link" } & ParentNavItem)
  | {
      readonly kind: "action";
      readonly key: string;
      readonly action: "language" | "logout";
      readonly tKey: string;
      readonly icon: Icon;
    };

/** Die vier Alltagsziele. Das fuenfte Feld der Bottom-Navigation ist "Mehr". */
export const PARENT_PRIMARY_NAV: readonly ParentNavItem[] = [
  {
    key: "start",
    href: "/parents",
    tKey: "start",
    icon: House,
  },
  {
    key: "children",
    href: "/parents/children",
    tKey: "children",
    icon: Users,
  },
  {
    key: "messages",
    href: "/parents/messages",
    tKey: "messages",
    icon: ChatCircleText,
    badge: "messages",
  },
  {
    key: "calendar",
    href: "/parents/calendar",
    tKey: "calendar",
    icon: CalendarBlank,
  },
];

/**
 * Hinter "Mehr". Alles, was nicht taeglich gebraucht wird.
 *
 * Der Ungelesen-Zaehler der Neuigkeiten wird auf das "Mehr"-Symbol
 * aufaddiert. Sonst bliebe ein ungelesener Aushang unsichtbar, weil sein
 * Eintrag im Sheet liegt und die Bottom-Navigation ihn nicht zeigt.
 */
export const PARENT_MORE_NAV: readonly ParentMoreItem[] = [
  {
    kind: "link",
    key: "news",
    href: "/parents/news",
    tKey: "news",
    icon: Megaphone,
    badge: "news",
    gate: "news",
  },
  {
    kind: "link",
    key: "mealPlan",
    href: "/parents/meal-plan",
    tKey: "mealPlan",
    icon: ForkKnife,
    gate: "mealPlan",
  },
  {
    kind: "link",
    key: "notifications",
    href: "/parents/settings",
    tKey: "notifications",
    icon: Bell,
  },
  {
    kind: "action",
    key: "language",
    action: "language",
    tKey: "language",
    icon: Translate,
  },
  {
    kind: "link",
    key: "enroll",
    href: "/parents/enroll",
    tKey: "enroll",
    icon: UserPlus,
  },
  {
    kind: "action",
    key: "logout",
    action: "logout",
    tKey: "logout",
    icon: SignOut,
  },
];

/** Das fuenfte Feld der Bottom-Navigation. Kein eigener href. */
export const PARENT_MORE_ENTRY = {
  key: "more",
  tKey: "more",
  icon: DotsThree,
} as const;

/** Zaehler, die auf das "Mehr"-Symbol aufaddiert werden. */
export const PARENT_MORE_BADGE_SOURCES: readonly ParentNavBadgeSource[] = [
  "news",
];
