// Types for the PageHeaderWithSearch component system

export interface PageHeaderWithSearchProps {
  // Header configuration
  title: string;
  badge?: {
    icon?: React.ReactNode;
    count: number;
    label?: string;
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

  // Custom action button (desktop only)
  actionButton?: React.ReactNode;

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
  type: 'buttons' | 'grid' | 'dropdown';
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
  className?: string;
}

export interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  onClear?: () => void;
  className?: string;
  size?: 'sm' | 'md' | 'lg';
}

export interface FilterButtonProps {
  label: string;
  options: FilterOption[];
  value: string | string[];
  onChange: (value: string | string[]) => void;
  type?: 'buttons' | 'dropdown';
  multiSelect?: boolean;
  className?: string;
}

export interface MobileFilterPanelProps {
  isOpen: boolean;
  onClose: () => void;
  filters: FilterConfig[];
  onApply?: () => void;
  onReset?: () => void;
}

export interface ActiveFilterChipsProps {
  filters: ActiveFilter[];
  onClearAll?: () => void;
  className?: string;
}

export interface NavigationTabsProps {
  items: TabItem[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
  className?: string;
}