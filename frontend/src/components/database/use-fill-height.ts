"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Misst, wie viel Höhe unter dem Element bis zur Unterkante des Bildschirms
 * bleibt, und liefert sie als CSS-Wert. Damit füllt eine Liste den Rest der
 * Seite, statt nach der letzten Zeile auf dem gemusterten Grund zu enden.
 *
 * Gemeinsame Messung für das Master-Detail-Layout und die einspaltige
 * Sammlung (`DatabaseListLayout`): beide sollen gleich hoch werden, sonst
 * springt die Fläche, wenn ein Register zwischen den Bauarten wechselt.
 */
export function useFillHeight<T extends HTMLElement>(bottomOffset = 32) {
  const ref = useRef<T>(null);
  const [height, setHeight] = useState<string>("100dvh");

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    const measure = () => {
      const top = Math.max(0, Math.round(node.getBoundingClientRect().top));
      const next = `calc(100dvh - ${top + bottomOffset}px)`;
      setHeight((prev) => (prev === next ? prev : next));
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(document.documentElement);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [bottomOffset]);

  return { ref, height };
}
