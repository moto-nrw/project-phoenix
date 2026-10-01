"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import type { SchoolSetupStepKey } from "~/lib/school-setup-api";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  SETUP_TOUR_BRANCHES,
  SETUP_TOURS,
  type SetupTour,
  type SetupTourBranch,
  type SetupTourStop,
} from "./setup-tours";

/** Nach dieser Zeit gilt eine Stelle als nicht vorhanden. */
const MISSING_AFTER_MS = 4000;
/** So lange darf die Person woanders sein, bevor die Tour endet. */
const LEAVE_AFTER_MS = 800;
/** Ohne sichtbare Seitenleiste öffnet die Tour die Seite schneller selbst. */
const NAV_MISSING_AFTER_MS = 1500;
const POLL_MS = 200;
/** Ein Klick öffnet oft erst ein Fenster; kurz warten, bevor es weitergeht. */
const ADVANCE_DELAY_MS = 250;

/** So viel der Stelle muss zu sehen sein, damit die Tour auf sie zeigt. */
const MIN_VISIBLE_PX = 8;

/**
 * Ist die Stelle wirklich zu sehen? Eingeklappte Bereiche der Seitenleiste
 * bleiben im Baum: Sie sind `inert` und werden von einem Elternelement mit
 * `overflow: hidden` auf Höhe 0 zugeschnitten. Beides zählt als unsichtbar,
 * ebenso ausgeblendete Elemente (etwa mobile Knöpfe auf dem Desktop).
 */
function isShown(element: Element): boolean {
  if (element.closest("[inert]")) return false;
  if (element.getClientRects().length === 0) return false;
  const rect = element.getBoundingClientRect();
  let top = rect.top;
  let bottom = rect.bottom;
  let left = rect.left;
  let right = rect.right;
  for (
    let parent = element.parentElement;
    parent && parent !== document.body;
    parent = parent.parentElement
  ) {
    const style = window.getComputedStyle(parent);
    if (style.visibility === "hidden" || style.display === "none") {
      return false;
    }
    // Nur Zuschneiden zählt. Ein Scrollbereich (auto/scroll) versteckt
    // nichts dauerhaft; die Tour scrollt die Stelle selbst ins Bild.
    const clips = [style.overflowX, style.overflowY].some(
      (value) => value === "hidden" || value === "clip",
    );
    if (!clips) continue;
    const clip = parent.getBoundingClientRect();
    top = Math.max(top, clip.top);
    bottom = Math.min(bottom, clip.bottom);
    left = Math.max(left, clip.left);
    right = Math.min(right, clip.right);
  }
  return bottom - top >= MIN_VISIBLE_PX && right - left >= MIN_VISIBLE_PX;
}

/** Die erste sichtbare Stelle zum Selektor. */
export function findVisibleTarget(selector: string): Element | null {
  for (const element of Array.from(document.querySelectorAll(selector))) {
    if (isShown(element)) return element;
  }
  return null;
}

/** Die erste Station auf der Seite selbst, nach dem Weg durch die Leiste. */
function firstPageStop(stops: readonly SetupTourStop[]): number {
  const index = stops.findIndex((stop) => !stop.nav);
  return index === -1 ? 0 : index;
}

/**
 * Beim Zurückgehen aus einem Fenster, etwa „Neues Kind“, schließt die Tour
 * das Fenster, wenn die vorige Station nicht darin liegt. Sonst läge es über
 * dem Knopf, der es geöffnet hat.
 */
function closeOverlayLeftBehind(
  shown: Element | null,
  previous: SetupTourStop,
): void {
  const overlay = shown?.closest('[role="dialog"]');
  if (!overlay || overlay.querySelector(previous.target)) return;
  const close = overlay.querySelector<HTMLElement>("[data-overlay-close]");
  close?.click();
}

interface TourState {
  step: SchoolSetupStepKey;
  index: number;
  /** Mit „Zurück“ erreicht: keine Station wird automatisch übersprungen. */
  reviewing?: boolean;
  /** Die Tour läuft in diesem Zweig, etwa im Import. */
  branch?: SetupTourBranch;
}

function definitionOf(tour: TourState): SetupTour | undefined {
  return tour.branch
    ? SETUP_TOUR_BRANCHES[tour.branch]
    : SETUP_TOURS[tour.step];
}

export interface ActiveTourStop {
  stop: SetupTourStop;
  index: number;
  total: number;
  /** null, solange die Stelle gesucht wird oder sie fehlt. */
  target: Element | null;
  /** Die Stelle ist nach längerem Warten nicht aufgetaucht. */
  missing: boolean;
  /** Gibt es eine Station, zu der „Zurück“ führt? */
  canGoBack: boolean;
}

/**
 * Steuert eine geführte Tour (#2832): öffnet die Seite des Schritts, sucht
 * die Stelle jeder Station und geht weiter, wenn die Person sie anklickt
 * oder „Weiter“ wählt.
 */
export function useSetupTour(
  onFinished: () => void,
  /** Die Person hat die Seite der Tour verlassen. */
  onLeft: (step: SchoolSetupStepKey) => void,
): {
  active: ActiveTourStop | null;
  start: (step: SchoolSetupStepKey) => void;
  next: () => void;
  back: () => void;
  stop: () => void;
} {
  // Über eine Ref, damit ein neues Router-Objekt die laufende Suche nicht
  // neu startet und das gefundene Ziel verwirft.
  const router = useTenantRouter();
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  }, [router]);
  const pathname = usePathname();
  const [tour, setTour] = useState<TourState | null>(null);
  // Das Suchergebnis gehört zu genau einer Station. So zeigt eine neue
  // Station nie auf die Stelle der vorigen, bevor ihre eigene gefunden ist.
  const [search, setSearch] = useState<{
    stop: SetupTourStop;
    element: Element | null;
    missing: boolean;
  } | null>(null);

  const definition = tour ? definitionOf(tour) : undefined;
  const stop = definition && tour ? definition.stops[tour.index] : undefined;
  const current = search && search.stop === stop ? search : null;
  // Stationen der Seitenleiste gelten überall. Alle anderen erst, wenn ihre
  // Seite geladen ist: Nach dem Klick in der Seitenleiste steht kurz noch
  // die alte Seite da, und deren gleichnamiger Knopf wäre das falsche Ziel.
  const onStopPage =
    stop !== undefined &&
    definition !== undefined &&
    (stop.nav === true ||
      (stop.pathPattern
        ? stop.pathPattern.test(pathname)
        : pathname.endsWith(definition.path)));
  const target = current?.element ?? null;
  const missing = current?.missing ?? false;

  const end = useCallback(() => {
    setTour(null);
    setSearch(null);
  }, []);

  const next = useCallback(() => {
    if (!tour) return;
    const stops = definitionOf(tour)?.stops ?? [];
    if (tour.index + 1 >= stops.length) {
      setTour(null);
      setSearch(null);
      onFinished();
      return;
    }
    setTour({ ...tour, index: tour.index + 1, reviewing: false });
  }, [tour, onFinished]);

  const onPage = definition ? pathname.endsWith(definition.path) : false;
  const pageStart = definition ? firstPageStop(definition.stops) : 0;

  // „Zurück“ geht immer eine Station zurück, auch in die Seitenleiste: So
  // sieht man noch einmal, wo man hingeklickt hat.
  const canGoBack = (tour?.index ?? 0) > 0;

  const back = useCallback(() => {
    if (!tour || tour.index === 0) return;
    const previous = definitionOf(tour)?.stops[tour.index - 1];
    if (previous) closeOverlayLeftBehind(target, previous);
    setTour({ ...tour, index: tour.index - 1, reviewing: true });
  }, [tour, target]);

  const start = useCallback(
    (step: SchoolSetupStepKey) => {
      const tourDefinition = SETUP_TOURS[step];
      if (!tourDefinition) return;
      // Wer schon auf der Seite eines Zweigs steht, etwa beim Import,
      // beginnt dort.
      const branch = tourDefinition.stops.find(
        (candidate) =>
          candidate.branch &&
          pathname.endsWith(SETUP_TOUR_BRANCHES[candidate.branch].path),
      )?.branch;
      if (branch) {
        setTour({ step, index: 0, branch });
        return;
      }
      // Die Tour beginnt in der Seitenleiste. Wer schon auf der Seite ist,
      // fängt direkt dort an.
      const index = pathname.endsWith(tourDefinition.path)
        ? firstPageStop(tourDefinition.stops)
        : 0;
      setTour({ step, index });
    },
    [pathname],
  );

  // Ist die Seite erreicht, entfallen die übrigen Stationen der Leiste, auch
  // wenn die Person einen anderen Weg genommen hat. Nicht aber, wenn sie
  // bewusst zurückgegangen ist, um sich den Weg noch einmal anzusehen.
  useEffect(() => {
    if (tour && !tour.reviewing && stop?.nav && onPage) {
      setTour({ ...tour, index: pageStart });
    }
  }, [tour, stop, onPage, pageStart]);

  // In den Zweig wechseln, sobald seine Seite offen ist, auch wenn die Person
  // nicht über die markierte Stelle dorthin kam.
  const branchPath = stop?.branch
    ? SETUP_TOUR_BRANCHES[stop.branch].path
    : null;
  useEffect(() => {
    if (tour && stop?.branch && branchPath && pathname.endsWith(branchPath)) {
      setTour({ step: tour.step, index: 0, branch: stop.branch });
      setSearch(null);
    }
  }, [tour, stop, branchPath, pathname]);

  // Die Stelle suchen, bis sie da ist; sie erscheint oft erst nach dem
  // Seitenwechsel oder wenn ein Fenster aufgeht.
  useEffect(() => {
    if (!stop || !tour || !definition) return undefined;
    // Stationen der Seitenleiste gelten überall. Alle anderen erst, wenn ihre
    // Seite geladen ist: Nach dem Klick in der Seitenleiste steht kurz noch
    // die alte Seite da, und deren gleichnamiger Knopf wäre das falsche Ziel.
    const onItsPage = onStopPage;
    // Gezählte Durchläufe statt Uhrzeit: die Wartezeit hängt nur am Takt.
    let polls = 0;
    let timer = 0;
    let shown: Element | null = null;
    /** true, sobald die Suche für diese Station beendet ist. */
    const check = (): boolean => {
      if (!onItsPage) return false;
      // Ist das Ergebnis der Station schon da (etwa die Vorschau nach dem
      // Hochladen), geht es weiter. Nicht beim Zurückgehen: sonst käme man
      // nie zurück zu dieser Station.
      if (
        !tour.reviewing &&
        stop.advanceWhenVisible &&
        findVisibleTarget(stop.advanceWhenVisible)
      ) {
        setTour({ ...tour, index: tour.index + 1 });
        return true;
      }
      // Ein gefundenes Ziel bleibt es nur, solange es da und zu sehen ist.
      // Rendert die Seite neu, wird es ersetzt; dann neu suchen, statt auf
      // ein Element zu zeigen, das es nicht mehr gibt.
      if (shown) {
        if (shown.isConnected && isShown(shown)) return false;
        shown = null;
        setSearch(null);
      }
      if (
        !tour.reviewing &&
        stop.skipWhenVisible &&
        findVisibleTarget(stop.skipWhenVisible)
      ) {
        setTour({ ...tour, index: tour.index + (stop.skipCount ?? 1) });
        return true;
      }
      const found = findVisibleTarget(stop.target);
      if (found) {
        shown = found;
        setSearch({ stop, element: found, missing: false });
        return false;
      }
      polls += 1;
      const waited = polls * POLL_MS;
      if (stop.nav && waited > NAV_MISSING_AFTER_MS) {
        // Keine Seitenleiste (Handy, eingeklappt): die Seite direkt öffnen.
        routerRef.current.push(definition.path);
        setTour({ ...tour, index: firstPageStop(definition.stops) });
        return true;
      }
      if (waited > MISSING_AFTER_MS) {
        setSearch({ stop, element: null, missing: true });
      }
      return false;
    };
    // Sofort prüfen: Steht die Stelle schon da, zeigt die Sprechblase ohne
    // Umweg über die Bildschirmmitte auf sie. Danach im Takt weiter prüfen,
    // auch nach dem Fund, bis die Station wechselt.
    if (!check()) {
      timer = window.setInterval(() => {
        if (check()) window.clearInterval(timer);
      }, POLL_MS);
    }
    return () => window.clearInterval(timer);
  }, [stop, tour, definition, pathname, onStopPage]);

  // Die Person hat die Seite der Station verlassen, etwa über einen Link in
  // der Seitenleiste. Die Tour sucht dort nichts mehr und
  // beendet sich, statt nur abgedunkelt stehen zu bleiben. Als verlassen gilt
  // nur, wer schon auf der Seite war und einen Moment woanders ist: Das Laden
  // der Seite nach einem Klick in der Seitenleiste ist kein Verlassen.
  const reachedStop = useRef<SetupTourStop | null>(null);
  const onLeftRef = useRef(onLeft);
  useEffect(() => {
    onLeftRef.current = onLeft;
  }, [onLeft]);
  useEffect(() => {
    if (!stop || !tour || stop.nav) return undefined;
    if (onStopPage) {
      reachedStop.current = stop;
      return undefined;
    }
    if (reachedStop.current !== stop) return undefined;
    const timer = window.setTimeout(() => {
      reachedStop.current = null;
      setTour(null);
      setSearch(null);
      onLeftRef.current(tour.step);
    }, LEAVE_AFTER_MS);
    return () => window.clearTimeout(timer);
  }, [stop, tour, onStopPage]);

  // Stationen mit „click“ gehen weiter, sobald die Person die Stelle anklickt.
  // Führt die Stelle in einen Zweig, geht die Tour dort weiter.
  useEffect(() => {
    if (!stop || !target) return undefined;
    const { branch } = stop;
    if (stop.advance !== "click" && !branch) return undefined;
    const onClick = (event: MouseEvent) => {
      if (!(event.target instanceof Node && target.contains(event.target))) {
        return;
      }
      if (branch) {
        setTour((current) =>
          current ? { step: current.step, index: 0, branch } : current,
        );
        setSearch(null);
        return;
      }
      window.setTimeout(next, ADVANCE_DELAY_MS);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [stop, target, next]);

  return {
    active:
      stop && definition && tour
        ? {
            stop,
            index: tour.index,
            total: definition.stops.length,
            target,
            missing,
            canGoBack,
          }
        : null,
    start,
    next,
    back,
    stop: end,
  };
}
