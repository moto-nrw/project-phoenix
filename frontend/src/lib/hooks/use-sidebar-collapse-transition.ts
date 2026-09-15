"use client";

import { useEffect, useRef, useState } from "react";

// Muss zur Dauer des Breiten-Slides in sidebar-geometry.ts passen: erst wenn
// die Leiste ihre Endbreite erreicht hat, dürfen die ausgeblendeten Texte aus
// dem Baum verschwinden. Etwas Luft, damit das Entfernen nie vor dem Ende der
// Blende passiert.
const WIDTH_TRANSITION_MS = 240;

/**
 * Fragt bei jedem Klappen neu ab, statt den Wert in einen Zustand zu legen:
 * die Bewegung hängt daran nur im Moment des Umschaltens, und ein zusätzlicher
 * Zustand würde beim ersten Rendern eine Abweichung zwischen Server und Client
 * erzeugen.
 */
function prefersReducedMotion(): boolean {
  if (typeof globalThis.matchMedia !== "function") return false;
  return globalThis.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * Steuert Bezeichnungen, Chevrons und Zähler während des Klappens (#2923).
 *
 * Zwei getrennte Angaben, weil beides zu verschiedenen Zeitpunkten gilt:
 *
 * - `labelsMounted`: sind die Texte im Baum? Beim Einklappen bleiben sie für
 *   die Dauer der Bewegung stehen und blenden aus; danach werden sie
 *   entfernt. Im eingeklappten Ruhezustand trägt der Streifen keine Texte —
 *   Vorleseprogramme lesen dort nur die Icon-Beschriftungen.
 * - `labelsVisible`: das Ziel der Blende. Beim Aufklappen werden die Texte
 *   ein Bild lang unsichtbar eingehängt und blenden erst danach ein, sonst
 *   erscheinen sie schlagartig statt mit der Breite mitzukommen.
 *
 * Bei `prefers-reduced-motion` gibt es keine Bewegung, auf die zu warten
 * wäre: die Breite springt, also wechseln auch die Texte sofort. Sonst
 * stünden nach dem Zuschnappen für eine Viertelsekunde leere Zeilen im
 * Streifen.
 *
 * Beim ersten Rendern gibt es keine Bewegung: der gespeicherte Zustand wird
 * direkt gerendert.
 */
export function useSidebarCollapseTransition(collapsed: boolean) {
  const [labelsMounted, setLabelsMounted] = useState(!collapsed);
  const [labelsVisible, setLabelsVisible] = useState(!collapsed);
  const previousCollapsed = useRef(collapsed);

  useEffect(() => {
    if (previousCollapsed.current === collapsed) return;
    previousCollapsed.current = collapsed;

    if (prefersReducedMotion()) {
      setLabelsMounted(!collapsed);
      setLabelsVisible(!collapsed);
      return;
    }

    if (collapsed) {
      // Ausblenden, dann ausbauen.
      setLabelsVisible(false);
      const timer = setTimeout(
        () => setLabelsMounted(false),
        WIDTH_TRANSITION_MS,
      );
      return () => clearTimeout(timer);
    }

    // Einbauen, dann im nächsten Bild einblenden.
    setLabelsMounted(true);
    const frame = requestAnimationFrame(() => setLabelsVisible(true));
    return () => cancelAnimationFrame(frame);
  }, [collapsed]);

  return { labelsMounted, labelsVisible };
}
