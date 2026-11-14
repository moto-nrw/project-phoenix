/**
 * Die Maße, die Seitenkopf und Bedienelemente des Kits miteinander teilen
 * (#6579). Was hier steht, nimmt das Kit von hier, statt es als Zahl zu
 * wiederholen: `TenantPage`, `TenantPageHeaderSkeleton`, `SegmentedControl`,
 * `OverflowMenu`, `CollectionViewSwitch`, `PlanningContextBar`, die Felder
 * und `TENANT-PAGE-SPEC.md` beziehen sich darauf.
 *
 * Die Werte sind ganze Tailwind-Klassen, keine Bausteine: Tailwind findet
 * eine Klasse nur, wenn sie wörtlich im Quelltext steht.
 */

// --- Seitenkopf ---------------------------------------------------------------

/**
 * Innenrand der Kopfkarte: 20 px, in der Dichte-Stufe `compact` und auf dem
 * Telefon 16 px.
 */
export const HEAD_PADDING = "compact:p-4 p-5 max-sm:p-4";

/**
 * Schriftgrad des Seitentitels. Der Desktop-Grad ist der Grundwert, Telefon
 * und Dichte-Stufe ziehen ihn je eine Stufe herunter (#2824). `leading-tight`
 * gehört dazu: Die Höhe der Titelzeile ist Schriftgrad mal Zeilenhöhe, und
 * der Ladekopf hält genau diese Höhe frei.
 */
export const HEAD_TITLE_SIZE = {
  /** 24 px, `compact` und Telefon 20 px. */
  standard: "compact:text-xl text-2xl leading-tight max-sm:text-xl",
  /** 28 px, `compact` und Telefon 24 px (Startseiten mit Begrüßung). */
  prominent: "compact:text-2xl text-[28px] leading-tight max-sm:text-2xl",
} as const;

/**
 * Eine Bedienhöhe im Seitenkopf: 40 px (`CONTROL_HEIGHT_CLASS.compact`), auf
 * dem Telefon 44 px, die Untergrenze für eine Touch-Fläche. Ohne diese
 * Klammer standen 32, 36 und 40 px nebeneinander, je nachdem welches
 * Kit-Bauteil eine Seite griff (gemessen auf /statistics). Der Selektor greift
 * auch verschachtelte Auslöser, deshalb Nachfahren und nicht nur Kinder.
 *
 * Teile eines zusammengesetzten Bedienelements (`CONTROL_PART`, die Segmente
 * eines `SegmentedControl`) nimmt er aus: Ihre Höhe ergibt sich mit der Spur
 * des Elements zur Bedienhöhe.
 */
export const HEAD_CONTROL_HEIGHT =
  "[&_button:not([data-control-part])]:h-10 [&_a[data-button-link]]:h-10 [&_select]:h-10 [&_input]:h-10 max-sm:[&_button:not([data-control-part])]:h-11 max-sm:[&_a[data-button-link]]:h-11 max-sm:[&_select]:h-11 max-sm:[&_input]:h-11";

/**
 * Dieselbe Bedienhöhe für ein einzelnes Element, das selbst den Platz eines
 * Bedienelements hält (Platzhalter im Ladekopf).
 */
export const HEAD_CONTROL_HEIGHT_SELF = "h-10 max-sm:h-11";

/**
 * Anordnung der Kopfaktionen. Ab `sm` stehen sie in einer Gruppe neben dem
 * Titel. Unter `sm` ist jede Zeile voll: Ein Textknopf bekommt eine eigene,
 * volle Zeile, ein reines Symbol-Bedienelement (`ICON_ONLY`) behält sein Maß
 * und bleibt in der Titel- und Statuszeile. Ein Symbol-Element, dessen Wurzel
 * ein `div` ist, wird `inline-flex`, damit die Text-Grundlinie keinen Abstand
 * unter den Knopf legt. Ein Menü hinter einem Textknopf („Herunterladen ⌄“)
 * steht in einer Hülle: Die Zeile ist voll, also füllt sein Knopf sie auch.
 */
export const HEAD_ACTIONS_LAYOUT =
  "[&>div[data-icon-only]]:inline-flex max-sm:contents max-sm:[&>*:not([data-icon-only])]:order-last max-sm:[&>*:not([data-icon-only])]:basis-full max-sm:[&>div:not([data-icon-only])>button]:w-full";

// --- Marker -------------------------------------------------------------------

/**
 * Kennzeichen eines reinen Symbol-Bedienelements (Kebab, `Button
 * size="icon"`, Ansichts-Umschalter). Das Element trägt es an seiner Wurzel;
 * ein Aufrufer braucht dafür keine Hülle.
 */
export const ICON_ONLY = { "data-icon-only": "" } as const;

/** `ICON_ONLY`, wenn `iconOnly` gilt, sonst nichts. */
export function iconOnlyMarker(iconOnly: boolean): { "data-icon-only"?: "" } {
  return iconOnly ? ICON_ONLY : {};
}

/**
 * Kennzeichen eines Teils, dessen Höhe das zusammengesetzte Bedienelement
 * bestimmt (Segment eines `SegmentedControl`). `HEAD_CONTROL_HEIGHT` lässt es
 * aus.
 */
export const CONTROL_PART = { "data-control-part": "" } as const;

// --- Bedienelemente -------------------------------------------------------------

/**
 * Das eine Größen-Vokabular der Felder und Auswahlen. Jedes Bedienelement
 * nimmt die Größen, die es braucht; ein Name bedeutet überall dieselbe Höhe.
 *
 * - `dense`:   36 px, Auswahl in dichten Zeilen
 * - `compact`: 40 px, wie `Button size="field"` und die Bedienhöhe im Kopf
 * - `touch`:   per Finger 44 px, mit der Maus 40 px: der Zeiger
 *              entscheidet, nicht die Fensterbreite
 * - `large`:   Höhe aus Polster und 16-px-Schrift, das große Formularfeld
 * - `natural`: Höhe aus Polster und 14-px-Schrift, der ursprüngliche
 *              Auslöser der Datumsauswahl in Filterleisten
 */
export type ControlSize = "dense" | "compact" | "touch" | "large" | "natural";

/** Die festen Höhen des Vokabulars. */
export const CONTROL_HEIGHT_CLASS = {
  dense: "h-9",
  compact: "h-10",
  touch: "h-10 pointer-coarse:h-11",
} as const satisfies Partial<Record<ControlSize, string>>;
