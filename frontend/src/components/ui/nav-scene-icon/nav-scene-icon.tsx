"use client";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import {
  MOTO_DUOTONE_TONES,
  MotoDuotoneIcon,
} from "~/components/ui/moto-duotone-icon";
import type { MotoDuotoneTone } from "~/lib/location-helper";
import { cn } from "~/lib/utils";
import { NAV_SCENES } from "./nav-scenes";

/** Länger als die längste Szene samt Verzögerung (ca. 700 ms). */
export const NAV_SCENE_PLAY_MS = 900;

const PLAY_CLASS = "ns-play";

/**
 * Spielt die Szene des Elements, sobald die Maus die Navigationszeile
 * betritt oder die Zeile per Tastatur den Fokus bekommt. Die Szene läuft
 * immer zu Ende, auch wenn die Maus vorher weitergeht: die Klasse
 * `ns-play` bleibt für `NAV_SCENE_PLAY_MS` stehen, statt an `:hover` zu
 * hängen.
 *
 * Die Zeile ist der nächste Link oder Knopf um das Icon. Gesperrte Zeilen
 * („Bald verfügbar") sind weder das eine noch das andere und spielen nichts.
 * Ein Tipp auf dem Tablet spielt nichts; bei „Bewegung reduzieren" auch nicht.
 */
function useNavScenePlay<T extends Element>() {
  const ref = useRef<T>(null);

  useEffect(() => {
    const element = ref.current;
    const row = element?.closest("a, button");
    if (!element || !row) return;

    let timer: ReturnType<typeof setTimeout> | undefined;
    const play = () => {
      if (element.classList.contains(PLAY_CLASS)) return;
      if (row.matches("[aria-disabled='true'], :disabled")) return;
      if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) {
        return;
      }
      element.classList.add(PLAY_CLASS);
      timer = setTimeout(
        () => element.classList.remove(PLAY_CLASS),
        NAV_SCENE_PLAY_MS,
      );
    };
    const onPointerEnter = (event: Event) => {
      if ((event as PointerEvent).pointerType === "mouse") play();
    };
    const onFocus = () => {
      // Nur Tastaturfokus spielt; ein Mausklick hat die Szene schon beim
      // Überfahren gespielt. Engines ohne `:focus-visible` werfen hier.
      try {
        if (row.matches(":focus-visible")) play();
      } catch {
        // ohne :focus-visible keine Szene beim Fokus, das Icon bleibt ruhig
      }
    };

    row.addEventListener("pointerenter", onPointerEnter);
    row.addEventListener("focus", onFocus);
    return () => {
      row.removeEventListener("pointerenter", onPointerEnter);
      row.removeEventListener("focus", onFocus);
      clearTimeout(timer);
      element.classList.remove(PLAY_CLASS);
    };
  }, []);

  return ref;
}

/** Die Szene zum Glyph, `undefined` wenn es keine gibt. */
function getNavScene(icon: PhosphorIcon) {
  return icon.displayName ? NAV_SCENES[icon.displayName] : undefined;
}

/**
 * Das Icon einer Seitenleisten-Zeile mit seiner Szene.
 *
 * Für jedes Phosphor-Glyph, das eine Seitenleiste zeigt, liegt in
 * `nav-scenes.tsx` eine nachgezeichnete Fassung, deren Teile sich beim
 * Überfahren einzeln bewegen. Aktiv trägt das Icon die Farbe seines
 * Konzepts, inaktiv die Farbe aus `className`. Ein Glyph ohne Szene
 * (nur außerhalb der Seitenleisten denkbar, der Test verbietet es dort)
 * hüpft als Ganzes.
 */
export function NavSceneIcon({
  icon,
  tone,
  active = false,
  size = 20,
  className,
}: Readonly<{
  icon: PhosphorIcon;
  /** Der Ton des Konzepts; färbt das Icon im Aktivzustand. */
  tone?: MotoDuotoneTone;
  active?: boolean;
  size?: number;
  className?: string;
}>) {
  const scene = getNavScene(icon);
  const svgRef = useNavScenePlay<SVGSVGElement>();
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, "");

  if (!scene) {
    const Glyph = icon;
    return (
      <NavSceneFallback className={className}>
        {active && tone ? (
          <MotoDuotoneIcon icon={icon} tone={tone} size={size} />
        ) : (
          <Glyph size={size} weight="regular" aria-hidden="true" />
        )}
      </NavSceneFallback>
    );
  }

  const outline = scene.filled
    ? { fill: "currentColor" }
    : {
        fill: "none",
        stroke: "currentColor",
        strokeWidth: 16,
        strokeLinecap: "round" as const,
        strokeLinejoin: "round" as const,
      };

  return (
    <svg
      ref={svgRef}
      viewBox="0 0 256 256"
      width={size}
      height={size}
      className={cn("ns-icon shrink-0", className)}
      style={
        active && tone ? { color: MOTO_DUOTONE_TONES[tone].primary } : undefined
      }
      data-nav-scene={icon.displayName}
      // Dasselbe Merkmal wie MotoDuotoneIcon: aktiv trägt das Icon den Ton
      // seines Konzepts.
      data-moto-duotone-tone={active && tone ? tone : undefined}
      aria-hidden="true"
      focusable="false"
      {...outline}
    >
      {scene.render(uid)}
    </svg>
  );
}

/**
 * Die Standardbewegung für Icons ohne Szene: das ganze Icon hüpft einmal.
 * Für die alten Pfad-Icons der Operator-Navigation, die kein Konzept tragen.
 */
export function NavSceneFallback({
  className,
  children,
}: Readonly<{ className?: string; children: ReactNode }>) {
  const ref = useNavScenePlay<HTMLSpanElement>();
  return (
    <span
      ref={ref}
      className={cn("ns-fallback inline-flex shrink-0", className)}
    >
      {children}
    </span>
  );
}
