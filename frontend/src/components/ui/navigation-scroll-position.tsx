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

function currentUrl() {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
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
 * je Pfad und holt sie nach. Ein Verlaufsschritt kann den Pfad bestätigen,
 * bevor `popstate` ankommt. Deshalb wartet der normale Reset bis zum nächsten
 * Frame; `popstate` ersetzt ihn dann noch ohne sichtbaren Sprung durch die
 * Wiederherstellung.
 *
 * Die Komponente steht in der Hülle vor dem Seiteninhalt. So laufen ihre Effekte
 * vor denen der Seite, und eine Seite oder Next darf danach gezielt woanders
 * hinscrollen (etwa zu einer Sprungmarke in der Adresse).
 */
export function NavigationScrollPosition() {
  const pathname = usePathname();
  const committedPathname = useRef(pathname);
  const committedUrl = useRef<string | undefined>(undefined);
  const lastCommitAt = useRef(Number.NEGATIVE_INFINITY);
  const traversalBeforeCommit = useRef(false);
  // Aus dem Scroll-Ereignis, nicht aus `window.scrollY` beim Wechsel: Dort hat
  // der Browser die Position schon auf die Höhe der neuen Seite begrenzt.
  const lastScrollY = useRef(0);
  const scrollPositions = useRef(new Map<string, number>());
  const resetFrame = useRef<number | undefined>(undefined);
  const stopRestore = useRef<(() => void) | undefined>(undefined);

  useEffect(() => {
    committedUrl.current = currentUrl();
    const handleScroll = () => {
      lastScrollY.current = window.scrollY;
    };
    const handleHistoryTraversal = () => {
      const target = window.location.pathname;
      const targetUrl = currentUrl();
      const beforeCommit = target !== committedPathname.current;
      const afterCommit =
        !beforeCommit &&
        targetUrl === committedUrl.current &&
        performance.now() - lastCommitAt.current < TRAVERSAL_COMMIT_WINDOW_MS;
      traversalBeforeCommit.current = beforeCommit;
      // Query- und Sprungmarkenwechsel sind kein Pfadwechsel. Nur eine genau
      // passende, gerade bestätigte Adresse kann dessen spätes `popstate` sein.
      if (!beforeCommit && !afterCommit) {
        if (resetFrame.current !== undefined) {
          cancelAnimationFrame(resetFrame.current);
          resetFrame.current = undefined;
        }
        return;
      }

      if (resetFrame.current !== undefined) {
        cancelAnimationFrame(resetFrame.current);
        resetFrame.current = undefined;
      }
      const position = scrollPositions.current.get(target);
      stopRestore.current?.();
      // Sprungmarken sind eine ausdrückliche Zielposition und gewinnen gegen
      // die gemerkte Position des Pfads.
      stopRestore.current =
        window.location.hash === "" && position !== undefined
          ? restoreScrollPosition(position)
          : undefined;
    };
    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("popstate", handleHistoryTraversal, {
      capture: true,
    });
    return () => {
      window.removeEventListener("scroll", handleScroll);
      window.removeEventListener("popstate", handleHistoryTraversal, {
        capture: true,
      });
      if (resetFrame.current !== undefined) {
        cancelAnimationFrame(resetFrame.current);
      }
      stopRestore.current?.();
    };
  }, []);

  useLayoutEffect(() => {
    if (committedPathname.current === pathname) return;
    if (resetFrame.current !== undefined) {
      cancelAnimationFrame(resetFrame.current);
      resetFrame.current = undefined;
    }
    const isTraversal = traversalBeforeCommit.current;
    if (!isTraversal) {
      scrollPositions.current.set(
        committedPathname.current,
        lastScrollY.current,
      );
    }
    committedPathname.current = pathname;
    committedUrl.current = currentUrl();
    lastCommitAt.current = performance.now();

    if (isTraversal) {
      // Bei schnellem Zurück-Zurück kann ein Zwischenziel bestätigt werden; die
      // Markierung gilt, bis der Router das letzte Ziel erreicht hat.
      traversalBeforeCommit.current = pathname !== window.location.pathname;
      return;
    }

    stopRestore.current?.();
    if (window.location.hash !== "") return;
    // Ein Verlaufsschritt kann den Pfad vor `popstate` bestätigen. Bis zum
    // nächsten Frame kann der Handler den Reset noch ohne sichtbaren Sprung
    // durch die Wiederherstellung ersetzen.
    resetFrame.current = requestAnimationFrame(() => {
      resetFrame.current = undefined;
      window.scrollTo({ top: 0, behavior: "instant" });
    });
  }, [pathname]);

  return null;
}
