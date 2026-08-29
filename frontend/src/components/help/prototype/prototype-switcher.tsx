"use client";

import { useCallback, useEffect } from "react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { Button } from "~/components/ui/button";
import { PROTOTYPE_VARIANTS, type PrototypeVariant } from "./prototype-data";

const VARIANTS = Object.keys(PROTOTYPE_VARIANTS) as PrototypeVariant[];

export function PrototypeSwitcher({
  current,
  onChange,
}: Readonly<{
  current: PrototypeVariant;
  onChange: (variant: PrototypeVariant) => void;
}>) {
  const cycle = useCallback(
    (direction: -1 | 1) => {
      const currentIndex = VARIANTS.indexOf(current);
      const nextIndex =
        (currentIndex + direction + VARIANTS.length) % VARIANTS.length;
      const next = VARIANTS[nextIndex];
      if (next) onChange(next);
    },
    [current, onChange],
  );

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target;
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        (target instanceof HTMLElement && target.isContentEditable)
      ) {
        return;
      }
      if (event.key === "ArrowLeft") cycle(-1);
      if (event.key === "ArrowRight") cycle(1);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [cycle]);

  if (process.env.NODE_ENV === "production") return null;

  return (
    <div className="fixed bottom-5 left-1/2 z-50 flex -translate-x-1/2 items-center gap-2 rounded-full border border-gray-700 bg-gray-950 px-2 py-2 text-white shadow-xl print:hidden">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="text-white hover:bg-gray-800 hover:text-white"
        aria-label="Vorherige Variante"
        onClick={() => cycle(-1)}
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      </Button>
      <p className="min-w-36 text-center text-sm font-medium">
        {current.toUpperCase()} – {PROTOTYPE_VARIANTS[current]}
      </p>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="text-white hover:bg-gray-800 hover:text-white"
        aria-label="Nächste Variante"
        onClick={() => cycle(1)}
      >
        <ArrowRight className="h-4 w-4" aria-hidden="true" />
      </Button>
    </div>
  );
}
