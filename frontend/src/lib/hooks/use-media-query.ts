"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

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

const noSubscription = () => () => undefined;
const unknownOnServer = () => null;

/**
 * Wie `useMediaQuery`, aber ohne Vorgabe vor der Hydration: `null` heißt
 * „noch unbekannt“. So kann ein Aufrufer bis dahin beide Darstellungen per
 * CSS umschalten (kein Flackern, Server- und Client-Render gleich) und danach
 * nur die passende im Dokument behalten. Bei einem Seitenwechsel im Client
 * steht der Wert schon im ersten Render fest. Ohne `matchMedia` (jsdom)
 * bleibt der Wert `null`.
 */
export function useResolvedMediaQuery(query: string): boolean | null {
  const subscribe = useCallback(
    (onChange: () => void) => {
      if (typeof window.matchMedia !== "function") return noSubscription();
      const mediaQuery = window.matchMedia(query);
      mediaQuery.addEventListener("change", onChange);
      return () => mediaQuery.removeEventListener("change", onChange);
    },
    [query],
  );
  const getSnapshot = useCallback(
    () =>
      typeof window.matchMedia === "function"
        ? window.matchMedia(query).matches
        : null,
    [query],
  );
  return useSyncExternalStore(subscribe, getSnapshot, unknownOnServer);
}

/** Tailwinds `sm`-Grenze: alles darunter ist ein Telefon-Format. */
export const BELOW_SM = "(max-width: 639px)";
/** Tailwinds `md`-Grenze: darunter wechseln Toasts in die Dialogdarstellung. */
export const BELOW_MD = "(max-width: 767px)";
/** Tailwinds `lg`-Grenze: darunter ist kein Platz für breite Wochenraster. */
export const BELOW_LG = "(max-width: 1023px)";
