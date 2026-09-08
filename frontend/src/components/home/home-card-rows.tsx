"use client";

import { useEffect, useState } from "react";

import Link from "~/components/ui/navigation-link";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";

/** Die aktuelle Berliner Uhrzeit als „HH:MM", minütlich nachgezogen. */
export function useBerlinClock(): string {
  const [now, setNow] = useState(() => berlinTime());
  useEffect(() => {
    const id = setInterval(() => setNow(berlinTime()), 30_000);
    return () => clearInterval(id);
  }, []);
  return now;
}

function berlinTime(): string {
  return new Intl.DateTimeFormat("de-DE", {
    timeZone: "Europe/Berlin",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date());
}

/**
 * Die Zeilen, die JETZT noch zählen: alles, was gerade läuft oder noch kommt.
 *
 * Eine Karte auf der Startseite hat Platz für drei Zeilen. Nimmt sie stumpf
 * die ersten drei des Tages, steht dort um 16 Uhr immer noch die
 * Frühbetreuung — ein Tagesplan, der um 8 Uhr stehen bleibt. Ist der Tag
 * vorbei, bleiben die letzten Zeilen stehen, statt die Karte zu leeren:
 * „heute war das" ist eine Antwort, eine leere Karte ist keine.
 */
export function upcomingFirst<T>(
  items: readonly T[],
  endTimeOf: (item: T) => string,
  now: string,
): readonly T[] {
  const firstRelevant = items.findIndex((item) => endTimeOf(item) > now);
  if (firstRelevant <= 0) return items;
  return items.slice(firstRelevant);
}

/**
 * Wie viele Zeilen eine Karte der Startseite zeigt (#2180).
 *
 * Die Karten stehen in einem Raster mit fester Zellenhöhe. Passt der Inhalt
 * nicht, darf er nicht scrollen: die letzte sichtbare Zeile wäre dann am
 * Kartenrand angeschnitten, und das liest sich, als liefe der Baustein aus
 * seiner Karte heraus. Stattdessen stehen so viele Zeilen da, wie ganz
 * hineinpassen, und darunter der Rest als eine Zeile mit Zahl und Weg.
 *
 * Auf einem Handy gibt es keine feste Zellenhöhe — dort wächst jede Karte mit
 * ihrem Inhalt, und gekappt wird nichts.
 */
export function useHomeCardRows<T>(
  items: readonly T[],
  maxRows: number,
): { shown: readonly T[]; hidden: number } {
  const isPhone = useMediaQuery(BELOW_SM);
  if (isPhone || items.length <= maxRows) {
    return { shown: items, hidden: 0 };
  }
  // Der Hinweis auf den Rest kostet selbst eine Zeile Platz.
  const shown = items.slice(0, Math.max(maxRows - 1, 1));
  return { shown, hidden: items.length - shown.length };
}

/** Die Zeile unter einer gekappten Liste: wie viel fehlt und wo es steht. */
export function HomeMoreRow({
  hidden,
  href,
  label,
}: {
  readonly hidden: number;
  readonly href: string;
  /** Was gezählt wird, im Plural — „Einsätze", „Hinweise", „Gruppen". */
  readonly label: string;
}) {
  if (hidden <= 0) return null;
  return (
    <Link
      href={href}
      className="mt-2 block shrink-0 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
    >
      Noch {hidden} {label} ansehen
    </Link>
  );
}
