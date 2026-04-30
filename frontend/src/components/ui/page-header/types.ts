// Types for the PageHeaderWithSearch component system

import type { OverflowMenuItem } from "./OverflowMenu";

export type { OverflowMenuItem } from "./OverflowMenu";

export interface PageHeaderWithSearchProps {
  // Header configuration
  readonly title: string;
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
   *   overflow on iPad-class viewports. The `MobileFilterPanel` covers
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

  // Layout options
  readonly className?: string;
}

interface TabItem {
  readonly id: string;
  readonly label: string;
  readonly count?: number;
}

export interface FilterConfig {
  readonly id: string;
  readonly label: string;
  readonly type: "buttons" | "grid" | "dropdown";
  readonly value: string | string[];
  readonly onChange: (value: string | string[]) => void;
  readonly options: FilterOption[];
  readonly multiSelect?: boolean;
  readonly className?: string;
}

interface FilterOption {
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
}

export interface MobileFilterPanelProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly filters: FilterConfig[];
  readonly onApply?: () => void;
  readonly onReset?: () => void;
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
