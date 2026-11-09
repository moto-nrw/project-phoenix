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

/**
 * Tailwinds Breakpoints, wie sie `--breakpoint-*` im Theme festlegt
 * (`tailwindcss/theme.css`, `globals.css` überschreibt sie nicht).
 * `tailwind-breakpoints.test.ts` liest beide Dateien und hält diese Werte
 * gleich. Sie stehen in rem: Ein Media-Query-`rem` folgt der Standardschrift
 * des Browsers, die Grenze verschiebt sich also mit ihr (20 px Schrift: `md`
 * bei 960 px).
 */
export const TAILWIND_BREAKPOINTS = {
  sm: "40rem",
  md: "48rem",
  lg: "64rem",
  xl: "80rem",
} as const;

export type TailwindBreakpoint = keyof typeof TAILWIND_BREAKPOINTS;

/** Trifft genau dort zu, wo Tailwinds `<bp>:`-Varianten greifen. */
export function atLeast(breakpoint: TailwindBreakpoint): string {
  return `(min-width: ${TAILWIND_BREAKPOINTS[breakpoint]})`;
}

/**
 * Das exakte Gegenstück zu `atLeast`: trifft auf jede Breite zu, auf der
 * Tailwinds `<bp>:`-Varianten nicht greifen, auch auf Bruchteile wie
 * 767,5 px. Ein `max-width` mit px-Abzug ließe dort eine Lücke und hinge
 * nicht an der Browserschrift.
 */
export function below(breakpoint: TailwindBreakpoint): string {
  return `not all and ${atLeast(breakpoint)}`;
}

/**
 * Von `from` (einschließlich) bis unter `until`, wie Tailwinds
 * `<from>:max-<until>:`. Bereichssyntax, weil `not` in einer Und-Verknüpfung
 * nur geklammert gilt.
 */
export function between(
  from: TailwindBreakpoint,
  until: TailwindBreakpoint,
): string {
  return `(${TAILWIND_BREAKPOINTS[from]} <= width < ${TAILWIND_BREAKPOINTS[until]})`;
}

/** Unter Tailwinds `sm`: ein Telefon-Format. */
export const BELOW_SM = below("sm");
/** Unter Tailwinds `md`: Toasts wechseln in die Dialogdarstellung. */
export const BELOW_MD = below("md");
/** Unter Tailwinds `lg`: kein Platz für breite Wochenraster. */
export const BELOW_LG = below("lg");
/** Unter Tailwinds `xl`. */
export const BELOW_XL = below("xl");
/** Ab Tailwinds `md`. */
export const AT_LEAST_MD = atLeast("md");
/** Ab Tailwinds `lg`: die Desktop-Seitenleiste ist sichtbar. */
export const AT_LEAST_LG = atLeast("lg");
/** Ab Tailwinds `xl`. */
export const AT_LEAST_XL = atLeast("xl");
