"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useLayoutEffect, useRef } from "react";

/** So lange darf eine Seite nach „Zurück“ nachladen, bis sie hoch genug ist. */
const RESTORE_TIMEOUT_MS = 3000;
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
function restoreScrollPosition(top: number, onStop: () => void): () => void {
  const deadline = performance.now() + RESTORE_TIMEOUT_MS;
  const pathname = window.location.pathname;
  let frame = 0;
  let stopped = false;
  const stop = () => {
    if (stopped) return;
    stopped = true;
    cancelAnimationFrame(frame);
    for (const type of USER_INPUT_EVENTS) {
      window.removeEventListener(type, stop);
    }
    onStop();
  };
  const step = () => {
    if (
      performance.now() >= deadline ||
      window.location.pathname !== pathname
    ) {
      stop();
      return;
    }
    if (top === 0 ? window.scrollY !== 0 : window.scrollY < top) {
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

function currentAddressWithHash() {
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
  const searchParams = useSearchParams();
  const search = searchParams?.toString() ?? "";
  const committedPathname = useRef(pathname);
  const committedUrl = useRef<string | undefined>(undefined);
  const awaitingPathTraversal = useRef(false);
  const traversalBeforeCommit = useRef(false);
  const traversalTargetPathname = useRef<string | undefined>(undefined);
  // Aus dem Scroll-Ereignis, nicht aus `window.scrollY` beim Wechsel: Dort hat
  // der Browser die Position schon auf die Höhe der neuen Seite begrenzt. Ein
  // geplanter Reset setzt ihn vor dem nächsten Frame bewusst auf 0. Ein Klick
  // vor der Link-Navigation ergänzt den Wert, falls dessen Ereignis noch in
  // der Warteschlange steht.
  const lastScrollY = useRef(0);
  const scrollPositions = useRef(new Map<string, number>());
  const resetFrame = useRef<number | undefined>(undefined);
  const stopRestore = useRef<(() => void) | undefined>(undefined);
  const isRestoring = useRef(false);

  useEffect(() => {
    committedUrl.current = currentAddressWithHash();
    const handleScroll = () => {
      lastScrollY.current = window.scrollY;
      // Nach einem Wechsel kann Next den Zielpfad schon bestätigen, bevor ein
      // spätes `popstate` den Verlaufsschritt kennzeichnet. Ein dann noch
      // eintreffendes Scroll-Ereignis gehört zur verlassenen Seite und darf
      // deren gemerkte Position nicht unter dem Zielpfad überschreiben.
      if (!isRestoring.current && !awaitingPathTraversal.current) {
        scrollPositions.current.set(
          committedPathname.current,
          lastScrollY.current,
        );
      }
    };
    // Die Capture-Phase liegt vor dem Link-Handler. So merken wir auch einen
    // programmgesteuerten Scrollstand, dessen `scroll`-Ereignis noch aussteht.
    const handleClick = () => {
      lastScrollY.current = window.scrollY;
    };
    const handleHistoryTraversal = () => {
      const target = window.location.pathname;
      const targetUrl = currentAddressWithHash();
      const beforeCommit = target !== committedPathname.current;
      const afterCommit =
        !beforeCommit &&
        awaitingPathTraversal.current &&
        targetUrl === committedUrl.current;
      if (beforeCommit && !isRestoring.current) {
        scrollPositions.current.set(
          committedPathname.current,
          lastScrollY.current,
        );
      }
      traversalBeforeCommit.current = beforeCommit;
      if (beforeCommit) {
        traversalTargetPathname.current = target;
      }
      // Query- und Sprungmarkenwechsel sind kein Pfadwechsel. Nur eine genau
      // passende, gerade bestätigte Adresse kann dessen spätes `popstate` sein.
      if (!beforeCommit && !afterCommit) {
        committedUrl.current = targetUrl;
        if (resetFrame.current !== undefined) {
          cancelAnimationFrame(resetFrame.current);
          resetFrame.current = undefined;
        }
        stopRestore.current?.();
        stopRestore.current = undefined;
        isRestoring.current = false;
        return;
      }

      awaitingPathTraversal.current = false;

      if (resetFrame.current !== undefined) {
        cancelAnimationFrame(resetFrame.current);
        resetFrame.current = undefined;
      }
      const position = scrollPositions.current.get(target);
      stopRestore.current?.();
      isRestoring.current = false;
      // Sprungmarken sind eine ausdrückliche Zielposition und gewinnen gegen
      // die gemerkte Position des Pfads.
      if (window.location.hash === "" && position !== undefined) {
        isRestoring.current = true;
        stopRestore.current = restoreScrollPosition(position, () => {
          isRestoring.current = false;
        });
      } else {
        stopRestore.current = undefined;
      }
    };
    const handleHashChange = () => {
      committedUrl.current = currentAddressWithHash();
      stopRestore.current?.();
      stopRestore.current = undefined;
      isRestoring.current = false;
    };
    const handleUserInput = () => {
      if (resetFrame.current === undefined) return;
      cancelAnimationFrame(resetFrame.current);
      resetFrame.current = undefined;
      awaitingPathTraversal.current = false;
    };
    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    document.addEventListener("click", handleClick, { capture: true });
    window.addEventListener("popstate", handleHistoryTraversal, {
      capture: true,
    });
    window.addEventListener("hashchange", handleHashChange);
    for (const type of USER_INPUT_EVENTS) {
      window.addEventListener(type, handleUserInput, { passive: true });
    }
    return () => {
      window.removeEventListener("scroll", handleScroll);
      document.removeEventListener("click", handleClick, { capture: true });
      window.removeEventListener("popstate", handleHistoryTraversal, {
        capture: true,
      });
      window.removeEventListener("hashchange", handleHashChange);
      for (const type of USER_INPUT_EVENTS) {
        window.removeEventListener(type, handleUserInput);
      }
      if (resetFrame.current !== undefined) {
        cancelAnimationFrame(resetFrame.current);
      }
      stopRestore.current?.();
      isRestoring.current = false;
    };
  }, []);

  useEffect(() => {
    const url = currentAddressWithHash();
    if (
      committedPathname.current === pathname &&
      committedUrl.current !== undefined &&
      committedUrl.current !== url
    ) {
      stopRestore.current?.();
      stopRestore.current = undefined;
      isRestoring.current = false;
    }
    committedUrl.current = url;
  }, [pathname, search]);

  useLayoutEffect(() => {
    if (committedPathname.current === pathname) return;
    if (resetFrame.current !== undefined) {
      cancelAnimationFrame(resetFrame.current);
      resetFrame.current = undefined;
    }
    const isTraversal =
      traversalBeforeCommit.current &&
      (pathname !== window.location.pathname ||
        pathname === traversalTargetPathname.current);
    if (!isTraversal) {
      scrollPositions.current.set(
        committedPathname.current,
        lastScrollY.current,
      );
    }
    committedPathname.current = pathname;
    committedUrl.current = currentAddressWithHash();

    if (isTraversal) {
      awaitingPathTraversal.current = false;
      // Bei schnellem Zurück-Zurück kann ein Zwischenziel bestätigt werden; die
      // Markierung gilt, bis der Router das letzte Ziel erreicht hat.
      traversalBeforeCommit.current = pathname !== window.location.pathname;
      return;
    }

    traversalBeforeCommit.current = false;
    traversalTargetPathname.current = undefined;
    stopRestore.current?.();
    isRestoring.current = false;
    if (window.location.hash !== "") return;
    // Ein Verlaufsschritt kann den Pfad vor `popstate` bestätigen. Bis zum
    // nächsten Frame kann der Handler den Reset noch ohne sichtbaren Sprung
    // durch die Wiederherstellung ersetzen.
    lastScrollY.current = 0;
    awaitingPathTraversal.current = true;
    resetFrame.current = requestAnimationFrame(() => {
      resetFrame.current = undefined;
      window.scrollTo({ top: 0, behavior: "instant" });
    });
  }, [pathname]);

  return null;
}
