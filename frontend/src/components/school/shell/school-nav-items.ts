/**
 * Die eine Navigationsliste des Schul-Portals ("moto schule", #2207).
 *
 * Seitennavigation und mobile Leiste rendern beide aus dieser Datei — ein
 * neues Ziel wird hier eingetragen, nicht an zwei Stellen. Analog zu
 * `parent/shell/parent-nav-items.ts`. Ein Eintrag kann einen Zähler tragen
 * (`badge`), den die beiden Leisten aus demselben Hook füllen.
 *
 * Ein Eintrag nennt sein Konzept, nicht sein Icon. Glyph und Ton kommen aus
 * der Konzeptdefinition (`~/lib/concepts`), damit das Schul-Portal dieselbe Bildsprache spricht wie
 * das übrige Produkt.
 */
import type { MotoConcept } from "~/lib/moto-concepts";
import { announcementsConcept } from "~/lib/concepts/announcements";
import { calendarConcept } from "~/lib/concepts/calendar";
import { classDayConcept } from "~/lib/concepts/classDay";
import { excusedConcept } from "~/lib/concepts/excused";
import { helpConcept } from "~/lib/concepts/help";
import { messagesConcept } from "~/lib/concepts/messages";
import { newsConcept } from "~/lib/concepts/news";
import { presentConcept } from "~/lib/concepts/present";

export interface SchoolNavItem {
  /** Stabiler Schlüssel für Listen und Tests. */
  readonly key: string;
  readonly href: string;
  readonly label: string;
  readonly concept: MotoConcept;
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
  readonly badge?: "teamChat" | "notices";
  /**
   * `true` für Ziele, die es nur gibt, solange die Schule die Funktion
   * nicht ausgeschaltet hat. Die Leisten blenden den Eintrag aus, solange das
   * nicht feststeht oder verneint ist — ein toter Link ist schlimmer als
   * ein fehlender.
   */
  readonly optional?: "teamChat";
}

/**
 * Die Alltagsziele: der heutige Tag einer Klasse, die Aufsichten, die diese
 * Lehrkraft heute selbst führt, die Nachrichten an die OGS (#2208) und die
 * Tagesinformationen der OGS-Leitung (#2208). Getrennte Namen mit getrennten
 * Aufgaben — die Klassenansicht ist die Übergabe nach Unterricht, die
 * Aufsichten sind der eigene Dienst danach, die Nachrichten der kurze Draht
 * zur OGS dazwischen, die Tagesinformationen das, was die OGS-Leitung allen
 * Lehrkräften mitteilt.
 */
export const SCHOOL_PRIMARY_NAV: readonly SchoolNavItem[] = [
  {
    key: "classDay",
    href: "/school",
    label: "Klassenansicht",
    concept: classDayConcept,
    portalPath: true,
  },
  {
    key: "supervisions",
    href: "/school/aufsichten",
    label: "Meine Aufsichten",
    concept: presentConcept,
    portalPath: true,
  },
  {
    key: "messages",
    href: "/school/nachrichten",
    label: "Nachrichten",
    concept: messagesConcept,
    portalPath: true,
    badge: "teamChat",
    optional: "teamChat",
  },
  {
    key: "notices",
    href: "/school/tagesinformationen",
    label: "Tagesinformationen",
    concept: announcementsConcept,
    portalPath: true,
    badge: "notices",
  },
  // Fehltage je Kind einer Klasse für Klassenbuch und Zeugnis (#4793).
  {
    key: "absenceList",
    href: "/school/fehlzeiten",
    label: "Fehlzeiten",
    concept: excusedConcept,
    portalPath: true,
  },
  // Briefe der Lehrkraft an die Eltern ihrer Klasse (#4800).
  {
    key: "parentLetters",
    href: "/school/elternbriefe",
    label: "Elternbriefe",
    concept: newsConcept,
    portalPath: true,
  },
  // Termine der OGS, Schließtage, Ferien und die eigenen Schultermine (#4798).
  {
    key: "calendar",
    href: "/school/kalender",
    label: "Kalender",
    concept: calendarConcept,
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
    // Mit `role` ueberspringt die Hilfe ihre Eingangsfrage: wer hier klickt,
    // ist eine Lehrkraft. Die drei OGS-Einstellungen aendern in der
    // Lehrkraft-Anleitung keinen Ablauf und bleiben deshalb weg.
    href: "/help?role=teacher",
    label: "Hilfe",
    concept: helpConcept,
    newTab: true,
  },
];
