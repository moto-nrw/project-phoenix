// Types for the PageHeaderWithSearch component system

import type { OverflowMenuItem } from "./OverflowMenu";
import type { MotoConceptKey } from "~/lib/moto-concepts";

export type { OverflowMenuItem } from "./OverflowMenu";

export interface PageHeaderWithSearchProps {
  // Header configuration
  readonly title: string;
  /**
   * Optional fachliches Konzept fuer die Titelzeile. Wenn gesetzt, rendert
   * der Titelbereich links eine graue Icon-Kachel (Header-Muster) mit dem
   * passenden {@link MotoConceptIcon}. Ohne Prop unveraendertes Verhalten.
   */
  readonly concept?: MotoConceptKey;
  readonly badge?: {
    readonly icon?: React.ReactNode;
    readonly count: number;
    readonly label?: string;
  };
  readonly statusIndicator?: {
    readonly color: "green" | "yellow" | "red" | "gray";
    readonly tooltip?: string;
  };

  // Optional navigation tabs (like in OGS groups or MyRoom)
  readonly tabs?: {
    readonly items: TabItem[];
    readonly activeTab: string;
    readonly onTabChange: (tabId: string) => void;
  };

  // Search configuration
  readonly search?: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly placeholder?: string;
    readonly className?: string;
    /** Forwarded to the underlying `<input>` (combobox role, ARIA wiring, disabled state, etc.). */
    readonly inputProps?: React.InputHTMLAttributes<HTMLInputElement>;
  };

  // Filter configuration
  readonly filters?: FilterConfig[];

  // Active filters for display
  readonly activeFilters?: ActiveFilter[];
  readonly onClearAllFilters?: () => void;

  // Custom action buttons
  readonly actionButton?: React.ReactNode; // Desktop action button (shown in tab row with full styling)
  readonly mobileActionButton?: React.ReactNode; // Mobile action button (compact version in tab row)

  /**
   * Bedienelement, das auf JEDER Breite in der Reiterzeile bleibt, rechts
   * neben den Reitern bzw. neben deren mobilem Dropdown.
   *
   * Der Unterschied zu `actionButton`/`mobileActionButton`: die beiden wandern
   * auf Mobil in die Titelzeile, sobald `title` gesetzt ist. Das passt für
   * einen Knopf, der zur Seite gehört, nicht aber für eine zweite Auswahl, die
   * neben den Reitern stehen soll (Anfragen: Reiter + Offen/Historie).
   * Erfordert `tabs`; ohne Reiter gibt es keine Reiterzeile.
   */
  readonly tabsRowAction?: React.ReactNode;

  /**
   * Optional kebab-menu (⋮) items rendered right of the tabs/title row.
   * Empty or undefined → no menu rendered. Used for rarely-touched actions
   * (export, "Gruppe übergeben", etc.) so the visible header stays calm.
   */
  readonly overflowMenu?: readonly OverflowMenuItem[];

  /**
   * Page-level primary action (e.g. a check-in mode trigger). When set, the
   * desktop search row splits into two rows: row 1 carries search +
   * primaryAction, row 2 carries filters + actionButton + kebab. Mobile is
   * unaffected — primary actions on mobile typically render as a floating
   * FAB at the page level, not inside this header.
   */
  readonly primaryAction?: React.ReactNode;

  /**
   * Viewport at which the desktop layout (inline filter dropdowns) takes
   * over from the mobile sheet pattern (search + filter button).
   *
   * - `"lg"` (default, 1024px): existing behaviour. Suitable for pages with
   *   ≤4 filters that fit comfortably on a tablet.
   * - `"xl"` (1280px): pages with 5+ filters where the inline row would
   *   overflow on iPad-class viewports. The `FilterPanel` covers
   *   tablet too — matches Stripe / Airbnb / Slack / Spotify pattern.
   *
   * Pages that opt into `"xl"` should also bump their tablet-only floating
   * FAB wrapper from `lg:hidden` to `xl:hidden` so the FAB and filter
   * sheet stay aligned to the same breakpoint.
   */
  readonly desktopFiltersFrom?: "lg" | "xl";

  /**
   * When true, the search row shrinks (transform-only) and gains a
   * backdrop-blur background once the page is scrolled past ~40px. Default
   * `false` so existing consumers are unaffected.
   */
  readonly compactOnScroll?: boolean;

  /**
   * How active filters are surfaced.
   *
   * - `"chips"` (default): the existing chip row below the search row.
   * - `"count"`: chip row is suppressed and the active count is shown as a
   *   numeric badge on the filter pill (mobile) and on a small inline pill
   *   in the desktop search row.
   */
  readonly activeFilterDisplay?: "chips" | "count";

  /**
   * Filter presentation. A single opt-in that swaps the whole filter
   * experience — there is no use case for mixing the two axes independently.
   *
   * - `"default"` (default): legacy inline desktop filter row + black-pill
   *   mobile sheet.
   * - `"quiet"`: one Filter button opens a shared popover (desktop *and*
   *   mobile) styled in the calm detail-modal language — softer controls and,
   *   when {@link filterSections} is supplied, `InfoSection` cards.
   */
  readonly filterVariant?: "default" | "quiet";

  /**
   * Optional grouping for the quiet filter panel. Each section claims the
   * filters whose `id` it lists; any filter not claimed lands in a trailing
   * "Weitere" section. Omit to render a flat list. The grouping is the
   * consumer's domain knowledge (which student/room/etc. filters belong
   * together), so it lives here — not baked into the shared panel.
   */
  readonly filterSections?: readonly FilterSection[];

  // Layout options
  readonly className?: string;
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

interface TabItem {
  readonly id: string;
  readonly label: string;
  readonly count?: number;
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

// Props for individual components
export interface PageHeaderProps {
  readonly title: string;
  /** See {@link PageHeaderWithSearchProps.concept}. */
  readonly concept?: MotoConceptKey;
  readonly badge?: {
    readonly icon?: React.ReactNode;
    readonly count: number;
    readonly label?: string;
  };
  readonly statusIndicator?: {
    readonly color: "green" | "yellow" | "red" | "gray";
    readonly tooltip?: string;
  };
  readonly actionButton?: React.ReactNode;
  /** Optional kebab-menu rendered after the action button on mobile. */
  readonly overflowMenu?: readonly OverflowMenuItem[];
  readonly className?: string;
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
}

export interface FilterPanelProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly filters: FilterConfig[];
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
  readonly filters: ActiveFilter[];
  readonly onClearAll?: () => void;
  readonly className?: string;
}

export interface NavigationTabsProps {
  readonly items: TabItem[];
  readonly activeTab: string;
  readonly onTabChange: (tabId: string) => void;
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
