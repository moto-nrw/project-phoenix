/**
 * Die eine Navigationsliste des Schul-Portals ("moto schule", #2207).
 *
 * Seitennavigation und mobile Leiste rendern beide aus dieser Datei — ein
 * neues Ziel wird hier eingetragen, nicht an zwei Stellen. Analog zu
 * `parent/shell/parent-nav-items.ts`, nur ohne Zähler und Sichtbarkeits-
 * schalter: eine Lehrkraft hat im Portal genau zwei Ziele.
 *
 * Ein Eintrag nennt sein Konzept, nicht sein Icon. Glyph und Ton kommen aus
 * `MOTO_CONCEPTS`, damit das Schul-Portal dieselbe Bildsprache spricht wie
 * das übrige Produkt.
 */
import type { MotoConceptKey } from "~/lib/moto-concepts";

export interface SchoolNavItem {
  /** Stabiler Schlüssel für Listen und Tests. */
  readonly key: string;
  readonly href: string;
  readonly label: string;
  readonly concept: MotoConceptKey;
  /**
   * Ziele außerhalb des Portals öffnen in einem neuen Tab, damit die
   * Klassenansicht beim Nachschlagen nicht verloren geht.
   */
  readonly newTab?: boolean;
  /**
   * `true` für Portal-Pfade, die auf dem Schul-Host ohne /school-Präfix
   * laufen. Die Hilfe ist host-unabhängig und wird nicht umgeschrieben.
   */
  readonly portalPath?: boolean;
}

/** Die Startseite des Schul-Hosts: der heutige Tag einer Klasse. */
export const SCHOOL_PRIMARY_NAV: readonly SchoolNavItem[] = [
  {
    key: "classDay",
    href: "/school",
    label: "Klassenansicht",
    concept: "classDay",
    portalPath: true,
  },
];

/**
 * Unten angeheftet: nicht der Alltag, sondern das Nachschlagen. Die Hilfe
 * liegt außerhalb des Portals (öffentliche /help-Strecke) und öffnet deshalb
 * in einem neuen Tab.
 */
export const SCHOOL_SECONDARY_NAV: readonly SchoolNavItem[] = [
  {
    key: "help",
    href: "/help",
    label: "Hilfe",
    concept: "help",
    newTab: true,
  },
];
