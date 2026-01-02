// Types for the PageHeaderWithSearch component system

export interface PageHeaderWithSearchProps {
  // Header configuration
  title: string;
  badge?: {
    icon?: React.ReactNode;
    count: number;
    label?: string;
  };
  statusIndicator?: {
    color: "green" | "yellow" | "red" | "gray";
    tooltip?: string;
  };

  // Optional navigation tabs (like in OGS groups or MyRoom)
  tabs?: {
    items: TabItem[];
    activeTab: string;
    onTabChange: (tabId: string) => void;
  };

  // Search configuration
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    className?: string;
  };

  // Filter configuration
  filters?: FilterConfig[];

  // Active filters for display
  activeFilters?: ActiveFilter[];
  onClearAllFilters?: () => void;

  // Custom action buttons
  actionButton?: React.ReactNode; // Desktop action button (shown in tab row with full styling)
  mobileActionButton?: React.ReactNode; // Mobile action button (compact version in tab row)

  // Layout options
  className?: string;
}

export interface TabItem {
  id: string;
  label: string;
  count?: number;
}

export interface FilterConfig {
  id: string;
  label: string;
  type: "buttons" | "grid" | "dropdown";
  value: string | string[];
  onChange: (value: string | string[]) => void;
  options: FilterOption[];
  multiSelect?: boolean;
  className?: string;
}

export interface FilterOption {
  value: string;
  label: string;
  icon?: string; // SVG path data for grid-style buttons
  count?: number;
}

export interface ActiveFilter {
  id: string;
  label: string;
  onRemove: () => void;
}

// Props for individual components
export interface PageHeaderProps {
  title: string;
  badge?: {
    icon?: React.ReactNode;
    count: number;
    label?: string;
  };
  statusIndicator?: {
    color: "green" | "yellow" | "red" | "gray";
    tooltip?: string;
  };
  actionButton?: React.ReactNode;
  className?: string;
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
