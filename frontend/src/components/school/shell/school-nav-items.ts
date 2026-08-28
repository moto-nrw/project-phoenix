/**
 * Die eine Navigationsliste des Schul-Portals ("moto schule", #2207).
 *
 * Seitennavigation und mobile Leiste rendern beide aus dieser Datei — ein
 * neues Ziel wird hier eingetragen, nicht an zwei Stellen. Analog zu
 * `parent/shell/parent-nav-items.ts`. Ein Eintrag kann einen Zähler tragen
 * (`badge`), den die beiden Leisten aus demselben Hook füllen.
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
  /**
   * Welcher Zähler neben dem Eintrag steht. Die Leisten holen den Wert aus
   * dem passenden Hook; die Liste selbst kennt keine Zahlen.
   */
  readonly badge?: "teamChat";
  /**
   * `true` für Ziele, die es nur gibt, wenn die Schule die Funktion
   * eingeschaltet hat. Die Leisten blenden den Eintrag aus, solange das
   * nicht feststeht oder verneint ist — ein toter Link ist schlimmer als
   * ein fehlender.
   */
  readonly optional?: "teamChat";
}

/**
 * Die Alltagsziele: der heutige Tag einer Klasse, die Aufsichten, die diese
 * Lehrkraft heute selbst führt, und die Nachrichten an die OGS (#2208).
 * Getrennte Namen mit getrennten Aufgaben — die Klassenansicht ist die
 * Übergabe nach Unterricht, die Aufsichten sind der eigene Dienst danach,
 * die Nachrichten der kurze Draht zur OGS dazwischen.
 */
export const SCHOOL_PRIMARY_NAV: readonly SchoolNavItem[] = [
  {
    key: "classDay",
    href: "/school",
    label: "Klassenansicht",
    concept: "classDay",
    portalPath: true,
  },
  {
    key: "supervisions",
    href: "/school/aufsichten",
    label: "Meine Aufsichten",
    concept: "present",
    portalPath: true,
  },
  {
    key: "messages",
    href: "/school/nachrichten",
    label: "Nachrichten",
    concept: "messages",
    portalPath: true,
    badge: "teamChat",
    optional: "teamChat",
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
