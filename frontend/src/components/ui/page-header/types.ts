// Typen des Seitenkopfs (`TenantPage`) und seiner Bausteine. Sie stehen hier
// genau einmal; die Bausteine importieren sie, nicht umgekehrt.

import type { ReactNode } from "react";

/**
 * Zähler-Plakette in der Such- und Filterzeile, z. B. die Zahl der gefundenen
 * Kinder.
 */
export interface PageHeaderBadge {
  readonly icon?: ReactNode;
  readonly count: number;
  readonly label?: string;
}

/** Suchfeld der Such- und Filterzeile. */
export interface PageHeaderSearch {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly placeholder?: string;
  /** Attribute des zugrunde liegenden Suchfelds, etwa `disabled` oder ARIA. */
  readonly inputProps?: React.InputHTMLAttributes<HTMLInputElement>;
  /** See `SearchBarProps.debounceMs` — keeps typing out of the page's state. */
  readonly debounceMs?: number;
  /** See `SearchBarProps.resetKey` — cancels a draft when a filter reset keeps the same value. */
  readonly resetKey?: string | number;
}

/**
 * Sammelt viele Filter hinter einer Filter-Schaltfläche, statt sie in die
 * Zeile zu legen: auf jeder Breite eine Schaltfläche mit Filterfläche, die
 * Zahl der aktiven Filter steht auf ihr statt als Chip-Reihe, und die
 * Desktop-Anordnung beginnt erst ab `xl` (1280 px). Ab etwa drei Filtern die
 * richtige Wahl — eine Zeile, die umbricht, ist keine Zeile mehr.
 */
export interface PageHeaderFilterPanel {
  /**
   * Überschriften der Filterfläche. Jede Gruppe nimmt die Filter, deren `id`
   * sie nennt; was keine Gruppe nimmt, steht unter „Weitere“. Ohne Angabe eine
   * flache Liste.
   */
  readonly sections?: readonly FilterSection[];
}

/**
 * A titled group of filters for the quiet panel. `icon` is rendered in the
 * section header (pass a `DetailIcons.*` element); `filterIds` are matched
 * against {@link FilterConfig.id}.
 */
export interface FilterSection {
  readonly title: string;
  readonly icon?: React.ReactNode;
  readonly filterIds: readonly string[];
}

export interface FilterConfig {
  readonly id: string;
  readonly label: string;
  readonly type: "buttons" | "grid" | "dropdown" | "custom";
  readonly value: string | string[];
  readonly onChange: (value: string | string[]) => void;
  readonly options: FilterOption[];
  readonly multiSelect?: boolean;
  /**
   * Trigger text of a multi-select whose selection is empty. Defaults to
   * `Alle {label}`; set it when that reads wrong ("Alle Stufe").
   */
  readonly emptyLabel?: string;
  /**
   * Trigger text of a multi-select once more than two values are chosen, so a
   * long selection does not overflow the trigger. Defaults to
   * `{n} {label} gewählt`.
   */
  readonly summaryLabel?: (count: number) => string;
  readonly className?: string;
  /**
   * For `type: "custom"` — the control rendered in place of the option
   * buttons/select, below the field label. Lets a consumer drop a bespoke
   * control (e.g. the Kindersuche planning-date chooser) into the filter set
   * so it lives *with* the other filters — in the quiet panel and grouped by
   * {@link FilterSection} — instead of as a stray row. `value`/`onChange`/
   * `options` are unused for custom filters; pass empty stubs. Because those
   * stubs carry no default, custom filters are excluded from the header's
   * default-state detection — report a non-default custom control through
   * `activeFilters` instead. Rendered by both the quiet `FilterPanel` and the
   * inline `DesktopFilters` row, so a custom control never silently disappears
   * on a consumer that uses the inline variant.
   */
  readonly render?: React.ReactNode;
}

export interface FilterOption {
  readonly value: string;
  readonly label: string;
  readonly icon?: string; // SVG path data for grid-style buttons
  readonly count?: number;
}

export interface ActiveFilter {
  readonly id: string;
  readonly label: string;
  readonly onRemove: () => void;
}

export interface SearchBarProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly placeholder?: string;
  readonly onClear?: () => void;
  readonly className?: string;
  readonly size?: "sm" | "md" | "lg";
  /** Forwarded to the underlying `<input>` (combobox role, ARIA wiring, etc.). */
  readonly inputProps?: React.InputHTMLAttributes<HTMLInputElement>;
  /**
   * Keep the typed text in the field and report it to `onChange` only after
   * this many milliseconds without a keystroke (#2975). Without it every
   * character re-renders the page that owns the term — on the Kindersuche that
   * was all 100 Kinderkarten per keystroke. `value` stays the source of truth:
   * an external change (a cleared filter chip) still overwrites the field.
   */
  readonly debounceMs?: number;
  /**
   * Changes whenever an external reset must win over a pending draft, even if
   * `value` already equals the last reported value. Without this signal a
   * controlled input cannot distinguish that reset from an unrelated render.
   */
  readonly resetKey?: string | number;
}

export interface FilterPanelProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly filters: readonly FilterConfig[];
  readonly onApply?: () => void;
  readonly onReset?: () => void;
  readonly applyLabel?: string;
  readonly placement?: "mobile" | "desktop";
  /** Initial measured trigger rect — drives the first paint (and SSR/tests). */
  readonly anchorRect?: FilterPanelAnchor | null;
  /**
   * Live trigger element. When provided, the panel re-positions itself on
   * scroll/resize so it tracks the trigger through native scroll and the
   * topbar's height transition.
   */
  readonly anchorRef?: React.RefObject<HTMLElement | null>;
  /**
   * Test hook on the dialog. The panel serves both the mobile sheet and the
   * desktop popover, so callers pass a placement-specific id.
   */
  readonly testId?: string;
  /**
   * `"quiet"` renders the panel in the calmer detail-modal language (softer
   * controls + blue-accent pills). Defaults to `"default"`, the existing look.
   */
  readonly variant?: "default" | "quiet";
  /**
   * Consumer-supplied grouping. When non-empty the panel renders the filters
   * as titled `InfoSection` cards; omit for a flat list.
   */
  readonly sections?: readonly FilterSection[];
}

export interface FilterPanelAnchor {
  readonly left: number;
  readonly bottom: number;
  readonly right: number;
}

export interface ActiveFilterChipsProps {
  readonly filters: readonly ActiveFilter[];
  readonly onClearAll?: () => void;
  readonly className?: string;
}

/**
 * Normalizes filter values to array format.
 * Handles single string values, arrays, and undefined.
 */
export function normalizeFilterValues(
  value: string | string[] | undefined,
): string[] {
  if (Array.isArray(value)) return value;
  if (value) return [value];
  return [];
}

export interface OverflowMenuItem {
  /** Visible label. */
  readonly label: string;
  /** One short line under the label that says what the entry does. */
  readonly description?: string;
  /** Optional left icon. */
  readonly icon?: ReactNode;
  /** Click handler. The menu closes automatically before this fires. */
  readonly onClick: () => void;
  /** Optional right-side hint, typically a count (e.g. "3"). */
  readonly badge?: string | number;
  /** Renders the item with a destructive (red) text colour. */
  readonly destructive?: boolean;
  /** When true, the item renders disabled and its onClick is suppressed. */
  readonly disabled?: boolean;
  /**
   * When set, the item renders as a link (next/link, or a plain anchor with
   * `target="_blank"` when `external` is true) instead of a button, keeping
   * native link semantics (middle-click, cmd-click). `onClick` still fires
   * after the menu closes — pass a no-op when only navigation is needed.
   */
  readonly href?: string;
  /** With `href`: open in a new tab via a plain anchor (noopener noreferrer). */
  readonly external?: boolean;
  /** Marks the current item in a navigational overflow menu, with a checkmark. */
  readonly selected?: boolean;
  /** Target of a guided tour (`data-setup-tour`), for an entry a tour points at. */
  readonly tourTarget?: string;
}

/** Non-interactive thin divider between item groups. */
export interface OverflowMenuSeparator {
  readonly kind: "separator";
}

/** Non-interactive uppercase label grouping the entries below it. */
export interface OverflowMenuHeader {
  readonly kind: "header";
  readonly label: string;
}

/** Single-select row with a checkmark shown when active (role="menuitemradio"). */
export interface OverflowMenuRadioItem {
  readonly kind: "radio";
  readonly label: string;
  /** One short line under the label that says what the choice means. */
  readonly description?: string;
  readonly checked: boolean;
  readonly onClick: () => void;
  readonly disabled?: boolean;
}

/**
 * On/off row with a checkmark shown while on (role="menuitemcheckbox"), for a
 * personal switch that belongs to the page, e.g. an e-mail on new entries.
 */
export interface OverflowMenuCheckboxItem {
  readonly kind: "checkbox";
  readonly label: string;
  readonly checked: boolean;
  readonly onClick: () => void;
  readonly disabled?: boolean;
  /**
   * Leaves the menu open after the toggle. For a list of switches that are
   * set together (which table columns to show): closing after every row
   * would make the user reopen the menu once per column.
   */
  readonly keepOpen?: boolean;
}

export type OverflowMenuEntry =
  | OverflowMenuItem
  | OverflowMenuHeader
  | OverflowMenuRadioItem
  | OverflowMenuCheckboxItem
  | OverflowMenuSeparator;
