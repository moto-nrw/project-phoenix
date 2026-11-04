"use client";

import { usePathname } from "next/navigation";
import { useEffect, useLayoutEffect, useRef } from "react";

/** So lange darf eine Seite nach „Zurück“ nachladen, bis sie hoch genug ist. */
const RESTORE_TIMEOUT_MS = 3000;
/** Abstand, in dem `popstate` noch zum eben bestätigten Pfadwechsel gehört. */
const TRAVERSAL_COMMIT_WINDOW_MS = 1000;
const USER_INPUT_EVENTS = [
  "wheel",
  "touchmove",
  "pointerdown",
  "keydown",
] as const;

/**
 * Hält das Dokument mindestens auf `top`, solange die Seite noch nachlädt. Beim
 * ersten Erreichen ist nichts entschieden: Die Ladehülle der Zielseite kann die
 * Position danach noch einmal auf ihre kurze Höhe drücken. Wer inzwischen selbst
 * scrollt oder klickt, behält die eigene Position.
 */
function restoreScrollPosition(top: number): () => void {
  const deadline = performance.now() + RESTORE_TIMEOUT_MS;
  const pathname = window.location.pathname;
  let frame = 0;
  const stop = () => {
    cancelAnimationFrame(frame);
    for (const type of USER_INPUT_EVENTS) {
      window.removeEventListener(type, stop);
    }
  };
  const step = () => {
    if (
      performance.now() >= deadline ||
      window.location.pathname !== pathname
    ) {
      stop();
      return;
    }
    if (window.scrollY < top) {
      window.scrollTo({ top, behavior: "instant" });
    }
    frame = requestAnimationFrame(step);
  };
  for (const type of USER_INPUT_EVENTS) {
    window.addEventListener(type, stop, { passive: true });
  }
  frame = requestAnimationFrame(step);
  return stop;
}

/**
 * Eine neu angesteuerte Seite beginnt oben (#5681).
 *
 * Next setzt die Position nur zurück, wenn der Anfang der neuen Seite außerhalb
 * des Sichtbereichs liegt. Die klebende Kopfzeile zählt dabei nicht mit: Liegt
 * der Seitenanfang hinter ihr, bleibt die alte Position stehen. Nur Pfadwechsel
 * zählen hier: Filter und Reiter in der Query (oft `scroll: false`) lassen die
 * Position in Ruhe.
 *
 * Zurück und Vor stellt der Browser wieder her, aber nicht zuverlässig: Steht
 * das Dokument auf einer kurzen Seite oben, begrenzt er die Position auf deren
 * Höhe und versucht es nicht erneut. Deshalb merkt sich die Hülle die Position
 * je Pfad und holt sie nach. Vor dem Bestätigen eines Wechsels lässt sich ein
 * Verlaufsschritt nicht erkennen: Im Browser kommt `popstate` meist erst danach.
 * Dann setzt der Bestätigungsschritt kurz nach oben, und `popstate` holt die
 * Position noch vor dem Zeichnen zurück.
 *
 * Die Komponente steht in der Hülle vor dem Seiteninhalt. So laufen ihre Effekte
 * vor denen der Seite, und eine Seite oder Next darf danach gezielt woanders
 * hinscrollen (etwa zu einer Sprungmarke in der Adresse).
 */
export function NavigationScrollReset() {
  const pathname = usePathname();
  const committedPathname = useRef(pathname);
  const lastCommitAt = useRef(Number.NEGATIVE_INFINITY);
  const traversalBeforeCommit = useRef(false);
  // Aus dem Scroll-Ereignis, nicht aus `window.scrollY` beim Wechsel: Dort hat
  // der Browser die Position schon auf die Höhe der neuen Seite begrenzt.
  const lastScrollY = useRef(0);
  const scrollPositions = useRef(new Map<string, number>());

  useEffect(() => {
    let stopRestore: (() => void) | undefined;
    const handleScroll = () => {
      lastScrollY.current = window.scrollY;
    };
    const handleHistoryTraversal = () => {
      const target = window.location.pathname;
      const beforeCommit = target !== committedPathname.current;
      const afterCommit =
        !beforeCommit &&
        performance.now() - lastCommitAt.current < TRAVERSAL_COMMIT_WINDOW_MS;
      traversalBeforeCommit.current = beforeCommit;
      // Ohne Pfadwechsel (nur Query oder der Wächter ungespeicherter Änderungen)
      // bleibt die Position, wo sie ist.
      if (!beforeCommit && !afterCommit) return;

      const position = scrollPositions.current.get(target);
      stopRestore?.();
      stopRestore = position ? restoreScrollPosition(position) : undefined;
    };
    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("popstate", handleHistoryTraversal);
    return () => {
      window.removeEventListener("scroll", handleScroll);
      window.removeEventListener("popstate", handleHistoryTraversal);
      stopRestore?.();
    };
  }, []);

  useLayoutEffect(() => {
    if (committedPathname.current === pathname) return;
    scrollPositions.current.set(committedPathname.current, lastScrollY.current);
    committedPathname.current = pathname;
    lastCommitAt.current = performance.now();

    if (traversalBeforeCommit.current) {
      // Bei schnellem Zurück-Zurück kann ein Zwischenziel bestätigt werden; die
      // Markierung gilt, bis der Router das letzte Ziel erreicht hat.
      traversalBeforeCommit.current = pathname !== window.location.pathname;
      return;
    }
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [pathname]);

  return null;
}
