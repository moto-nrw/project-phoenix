"use client";

import { useCallback, useRef } from "react";

/**
 * Merkt sich, wo jemand stand, bevor eine Liste durch eine Detailansicht
 * ersetzt wurde, und stellt beides wieder her: die Scroll-Position und den
 * Tastatur-Fokus auf der Zeile, die geöffnet wurde (#2267).
 *
 * Ohne das landet man nach „Zur Liste" oben auf der Seite und muss sich mit
 * der Tastatur erneut durch die ganze Liste arbeiten.
 */
export function useReturnFocus() {
  const elementIDRef = useRef<string | null>(null);
  const scrollYRef = useRef(0);

  const remember = useCallback((elementID: string) => {
    elementIDRef.current = elementID;
    scrollYRef.current = typeof window === "undefined" ? 0 : window.scrollY;
  }, []);

  const restore = useCallback(() => {
    if (typeof window === "undefined") return;
    const elementID = elementIDRef.current;
    const scrollY = scrollYRef.current;
    // Erst im nächsten Frame: die Liste ist zum Aufrufzeitpunkt noch nicht
    // wieder im DOM, ein sofortiges focus() würde ins Leere greifen.
    window.requestAnimationFrame(() => {
      window.scrollTo({ top: scrollY });
      if (!elementID) return;
      document.getElementById(elementID)?.focus();
    });
  }, []);

  return { remember, restore };
}
