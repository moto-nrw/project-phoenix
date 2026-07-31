"use client";

import { useEffect, useState } from "react";

/**
 * Verfolgt eine CSS-Media-Query im Client.
 *
 * Gedacht für die Fälle, in denen ein Breakpoint mehr entscheidet als
 * Sichtbarkeit — wenn also zwei Darstellungen NICHT beide ins Dokument gehören
 * und per `hidden`/`lg:block` weggeblendet werden dürfen. Beide zu rendern
 * verdoppelt Beschriftungen, IDs und die Tabulator-Reihenfolge: Screenreader
 * lesen jede Zeile zweimal vor, und die Tastaturnavigation läuft durch eine
 * unsichtbare Kopie. Für reine Sichtbarkeit bleiben Tailwind-Klassen der
 * einfachere und flackerfreie Weg.
 *
 * Vor dem ersten Effect ist das Ergebnis `false` — bewusst, damit Server- und
 * Client-Render übereinstimmen und React keine Hydrations-Abweichung meldet.
 * Aufrufer legen deshalb die Desktop-Variante als `false`-Fall aus.
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(false);

  useEffect(() => {
    const mediaQuery = window.matchMedia(query);
    const update = () => setMatches(mediaQuery.matches);
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, [query]);

  return matches;
}

/** Tailwinds `sm`-Grenze: alles darunter ist ein Telefon-Format. */
export const BELOW_SM = "(max-width: 639px)";
/** Tailwinds `md`-Grenze: darunter zählt die Darstellung als Telefonformat. */
export const BELOW_MD = "(max-width: 767px)";
/** Tailwinds `lg`-Grenze: darunter ist kein Platz für breite Wochenraster. */
export const BELOW_LG = "(max-width: 1023px)";
