// Types for the PageHeaderWithSearch component system

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

  // Layout options
  readonly className?: string;
}

export interface TabItem {
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
