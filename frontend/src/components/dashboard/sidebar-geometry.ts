/**
 * Das gemeinsame Raster der Seitenleiste (#2923).
 *
 * Ein- und ausgeklappte Leiste sind dieselben Elemente — nur die Breite
 * ändert sich. Damit beim Umschalten nichts springt, kommen Höhe, Abstand,
 * Ausrichtung und Rundung jeder Zeile aus dieser Datei, statt in
 * `sidebar.tsx` und `sidebar-accordion-section.tsx` je eigen ausgeschrieben
 * zu werden.
 *
 * Die Maße hängen voneinander ab; die Rechnung steht bei den Konstanten:
 *
 *   ausgeklappt   |<-10->|<-12->[icon 20]<-12->Bezeichnung ...      |
 *   eingeklappt   |<-10->|<-12->[icon 20]<-12->|
 *                        ^ Icon-Mitte 32px = Mitte der 64px-Leiste
 *
 * Weil Innenabstand der Navigation (10px) und Innenabstand der Zeile (12px)
 * in beiden Zuständen gleich sind, bleibt die Icon-Mitte über die ganze
 * Breitenänderung hinweg exakt an derselben Stelle: 10 + 12 + 10 = 32px,
 * also genau die Mitte des schmalen Streifens (64px / 2). Wer eines der
 * beiden Maße ändert, verschiebt die Icons beim Klappen wieder.
 */

/** Breite der ausgeklappten Leiste (256px). */
export const SIDEBAR_WIDTH_EXPANDED = "w-64";
/** Breite des eingeklappten Icon-Streifens (64px). */
export const SIDEBAR_WIDTH_COLLAPSED = "w-16";

/**
 * Breiten-Slide beim Klappen (#2825). Aussenhülle und Inhalt tragen dieselbe
 * Dauer und Kurve, damit die Zeilen mit der Leiste mitwandern statt am Anfang
 * oder Ende ein zweites Mal zu springen. `motion-safe` respektiert
 * prefers-reduced-motion: dort wechselt die Breite ohne Bewegung.
 */
export const SIDEBAR_WIDTH_TRANSITION =
  "motion-safe:transition-[width] motion-safe:duration-200 motion-safe:ease-in-out";

/** Innenabstand jeder Navigationsspalte (10px, siehe Rechnung oben). */
export const SIDEBAR_NAV_PADDING = "p-2.5";
/** Vertikaler Abstand zwischen zwei Zeilen (4px). */
export const SIDEBAR_NAV_GAP = "space-y-1";

/**
 * Grundriss jeder Navigationszeile: 40px hoch, 12px Innenabstand, gleiche
 * Rundung für Links und Bereichs-Schalter. Ohne Breakpoint-Varianten — ein
 * Raster für alle Desktop-Breiten, sonst wechselt die Zeilenhöhe zwischen
 * 1024px und 1280px und das Klappen springt genau dort (#2923).
 */
// overflow-hidden hält Chevron und Zähler während der Bewegung in der Zeile,
// statt sie über die Kante der schrumpfenden Leiste laufen zu lassen. Der
// Fokusring liegt deshalb innen (ring-inset), damit er nicht abgeschnitten
// wird — Links und Bereichs-Schalter tragen denselben Ring wie der
// Kit-Button.
const ROW_BASE =
  "group relative flex h-10 w-full items-center overflow-hidden rounded-lg px-3 text-left text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-inset focus-visible:outline-none";

const ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const ROW_INACTIVE =
  "font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900";
const ROW_DISABLED = "font-medium text-gray-400";

/**
 * Zeilenklassen für Links und Bereichs-Schalter. Ein Aufrufpunkt für beide,
 * damit Aktiv-Zustand, Hover und Fokus überall gleich aussehen.
 */
export function sidebarRowClasses(options?: {
  readonly isActive?: boolean;
  readonly isDisabled?: boolean;
}): string {
  if (options?.isDisabled)
    return `${ROW_BASE} ${ROW_DISABLED} cursor-not-allowed`;
  return `${ROW_BASE} ${options?.isActive ? ROW_ACTIVE : ROW_INACTIVE}`;
}

/** 20px-Icon, feste Größe in beiden Zuständen. */
export const SIDEBAR_ICON_CLASSES = "h-5 w-5 shrink-0 transition-colors";

/**
 * Die Bezeichnung neben dem Icon. Sie blendet mit der Breitenänderung aus
 * (kurze Blende, damit der Text nicht sichtbar "aufgefressen" wird) und beim
 * Aufklappen leicht verzögert wieder ein. `truncate` hält den Text
 * einzeilig — er darf während der Bewegung nicht umbrechen.
 */
export function sidebarLabelClasses(isVisible: boolean): string {
  return `ml-3 min-w-0 flex-1 truncate motion-safe:transition-opacity motion-safe:duration-150 ${
    isVisible ? "opacity-100 motion-safe:delay-75" : "opacity-0"
  }`;
}

/**
 * Unterpunkte richten sich an der Bezeichnung der Elternzeile aus:
 * 10px Navigationsabstand + 12px Zeilenabstand + 20px Icon + 12px = 54px,
 * abzüglich des Navigationsabstands also 44px (pl-11).
 */
export const SIDEBAR_SUB_ITEM_CLASSES =
  "flex h-8 items-center justify-between overflow-hidden rounded-lg pr-3 pl-11 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-inset focus-visible:outline-none";
