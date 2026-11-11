/**
 * Das gemeinsame Raster der Seitenleiste (#2923).
 *
 * Ein- und ausgeklappte Leiste sind dieselben Elemente — nur die Breite
 * ändert sich. Damit beim Umschalten nichts springt, kommen Höhe, Abstand,
 * Ausrichtung und Rundung jeder Zeile aus dieser Datei. Seit #5975 gilt das
 * Raster für die Leisten aller Portale (`sidebar-row.tsx`), nicht nur für
 * die einklappbare Tenant-Leiste.
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
 * Rückmeldung jeder anklickbaren Zeile: Farbe und Hintergrund blenden in
 * 150ms über, beim Drücken gibt die Zeile kurz nach (`motion-safe` lässt das
 * bei prefers-reduced-motion weg). Gesperrte Zeilen tragen kein Nachgeben.
 *
 * `--nav-row-bg` ist die aktuelle Hintergrundfarbe der Zeile. Die Szenen der
 * Icons (`NavSceneIcon`) füllen damit Teile, die etwas dahinter verdecken
 * (Sprechblasen, Ablage, Buchseite); ohne sie stünde dort beim Überfahren
 * ein weißer Fleck auf grauem Grund.
 */
const ROW_TRANSITION =
  "transition-[color,background-color,scale] duration-150 ease-out";
const ROW_PRESS = "motion-safe:active:scale-[0.98]";
const ROW_BG_IDLE =
  "[--nav-row-bg:var(--color-white)] hover:[--nav-row-bg:var(--color-gray-50)]";
const ROW_BG_ACTIVE = "[--nav-row-bg:var(--color-gray-100)]";

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
// justify-start steht ausdrücklich hier: der Bereichs-Schalter ist ein
// Kit-Button, und dessen Grundklassen zentrieren den Inhalt. Beide Zeilenarten
// beginnen links am Icon.
const ROW_BASE = `group relative flex h-10 w-full items-center justify-start overflow-hidden rounded-lg px-3 text-left text-sm ${ROW_TRANSITION} focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-inset focus-visible:outline-none`;

const ROW_ACTIVE = `bg-gray-100 font-semibold text-gray-900 ${ROW_BG_ACTIVE} ${ROW_PRESS}`;
const ROW_INACTIVE = `font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900 ${ROW_BG_IDLE} ${ROW_PRESS}`;
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

/**
 * Kopfzeile einer Gruppe (#2826): dieselbe Breite, derselbe Innenabstand
 * und dieselbe Icon-Spalte wie eine Navigationszeile, aber 32px hoch und
 * ohne Aktiv-Markierung — sie ist ein Schalter für den Block darunter,
 * keine Seite. Im Streifen bleibt nur ihr Icon übrig; weil px-3 gleich
 * ist, sitzt es auf derselben Achse wie die Zeilen-Icons.
 */
export const SIDEBAR_GROUP_HEADING_CLASSES = `group relative flex h-8 w-full items-center justify-start overflow-hidden rounded-lg px-3 text-left text-gray-600 ${ROW_TRANSITION} ${ROW_PRESS} ${ROW_BG_IDLE} hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-inset focus-visible:outline-none`;

/** 20px-Icon, feste Größe in beiden Zuständen. */
export const SIDEBAR_ICON_CLASSES = "h-5 w-5 shrink-0 transition-colors";

/**
 * Die Bezeichnung neben dem Icon. Sie blendet mit der Breitenänderung aus
 * (kurze Blende, damit der Text nicht sichtbar "aufgefressen" wird) und beim
 * Aufklappen leicht verzögert wieder ein. `truncate` hält den Text
 * einzeilig — er darf während der Bewegung nicht umbrechen.
 */
export function sidebarLabelClasses(
  isVisible: boolean,
  afterIcon = true,
): string {
  // Ohne Icon davor (Unterpunkt) fällt der Abstand zum Icon weg.
  return `${afterIcon ? "ml-3 " : ""}min-w-0 flex-1 truncate motion-safe:transition-opacity motion-safe:duration-150 ${
    isVisible ? "opacity-100 motion-safe:delay-75" : "opacity-0"
  }`;
}

/**
 * Das Icon einer Gruppen-Kopfzeile trägt die Farbe ihrer Beschriftung, wie die
 * Zeilen darunter; nur eine zugeklappte Gruppe mit der aktuellen Seite darin
 * hebt es an, damit man im Streifen sieht, wo man ist.
 */
export function sidebarGroupIconClasses(highlighted: boolean): string {
  return highlighted
    ? `${SIDEBAR_ICON_CLASSES} text-gray-900`
    : SIDEBAR_ICON_CLASSES;
}

/**
 * Die Zeilen einer Gruppe stehen eingerückt hinter einer Linie, die unter dem
 * Gruppen-Icon beginnt: 12px Zeilenabstand + 20px Icon / 2 − 7px = 15px. Im
 * Streifen (64px) gibt es dafür keinen Platz; die Einrückung gleitet dort mit
 * derselben Bewegung auf null wie die Breite.
 */
export function sidebarGroupBodyClasses(collapsed: boolean): string {
  return `${SIDEBAR_NAV_GAP} pt-1 motion-safe:transition-[margin-left,border-color] motion-safe:duration-200 motion-safe:ease-in-out ${
    collapsed
      ? "ml-0 border-l border-transparent"
      : "ml-[15px] border-l border-gray-200"
  }`;
}

/**
 * Beiwerk einer Zeile (Schloss, Zähler, Chevron) blendet mit der
 * Bezeichnung aus und ein.
 */
export function sidebarFadeClasses(visible: boolean): string {
  return `motion-safe:transition-opacity motion-safe:duration-150 ${visible ? "opacity-100" : "opacity-0"}`;
}

/** Der Chevron einer klappbaren Zeile; offen zeigt er nach oben. */
export function sidebarChevronClasses(open: boolean, visible: boolean): string {
  return `ml-2 h-4 w-4 shrink-0 text-gray-400 motion-safe:transition-[transform,opacity] motion-safe:duration-200 ${open ? "rotate-180" : ""} ${visible ? "opacity-100" : "opacity-0"}`;
}

/**
 * Im Streifen sitzt der Zähler als Punkt auf der Ecke des Icons. Die
 * Kopfzeile einer Gruppe ist 8px niedriger als eine Zeile (h-8 statt h-10):
 * `top-0` statt `top-1` hält den Punkt im selben Abstand zum Icon.
 */
export const SIDEBAR_RAIL_BADGE_CLASSES = "absolute top-1 right-1";
export const SIDEBAR_GROUP_HEADING_RAIL_BADGE_CLASSES =
  "absolute top-0 right-1";

/**
 * Unterpunkte richten sich an der Bezeichnung der Elternzeile aus:
 * 10px Navigationsabstand + 12px Zeilenabstand + 20px Icon + 12px = 54px,
 * abzüglich des Navigationsabstands also 44px (pl-11).
 */
const SUB_ITEM_BASE = `flex h-8 items-center justify-between overflow-hidden rounded-lg pr-3 pl-11 text-sm ${ROW_TRANSITION} motion-safe:active:scale-[0.98] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-inset focus-visible:outline-none`;
const SUB_ITEM_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const SUB_ITEM_INACTIVE =
  "font-medium text-gray-500 hover:bg-gray-50 hover:text-gray-700";

/** Klassen eines Unterpunkts unter einem Akkordeon-Bereich. */
export function sidebarSubItemClasses(isActive: boolean): string {
  return `${SUB_ITEM_BASE} ${isActive ? SUB_ITEM_ACTIVE : SUB_ITEM_INACTIVE}`;
}

/**
 * Ab dieser Fensterposition klebt die Leiste: unter der Kopfzeile (73 px), mit
 * einem festen Streifen darüber 121 px, mit der Abendzeile der Demo 153 px.
 */
export type PortalSidebarTopOffset = 73 | 121 | 153;

/**
 * Wo die Seitenleiste klebt: Ein fester Streifen (Demo, Mitarbeiter-Vorschau,
 * h-12) schiebt die Kopfzeile um 48 px nach unten (#2893, #3467), die Abendzeile
 * der Demo um weitere 32 px (#3921). Ohne das schöbe sich die Kopfzeile beim
 * Scrollen über die obersten Einträge.
 */
export function portalSidebarTopOffset(
  stripActive: boolean,
  eveningRow = false,
): PortalSidebarTopOffset {
  if (eveningRow) return 153;
  return stripActive ? 121 : 73;
}
