// Database Entity Configuration Types

import type { ReactNode } from 'react';
import type { DatabaseTheme } from '@/components/ui/database/themes';

// Field types supported by the database forms
export type FieldType = 
  | 'text' 
  | 'email' 
  | 'password' 
  | 'textarea' 
  | 'select' 
  | 'multiselect'
  | 'checkbox' 
  | 'custom'
  | 'number';

// Base field configuration
export interface FieldConfig {
  name: string;
  label: string;
  type: FieldType;
  required?: boolean;
  placeholder?: string;
  helperText?: string;
  validation?: (value: any) => string | null;
  // For select/multiselect fields - supports both sync and async options
  options?: Array<{ value: string; label: string }> | (() => Promise<Array<{ value: string; label: string }>>);
  // For custom fields
  component?: React.ComponentType<any>;
  // Grid layout
  colSpan?: 1 | 2;
  autoComplete?: string;
  // For number fields
  min?: number;
  max?: number;
}

// Section configuration for forms
export interface SectionConfig {
  title: string;
  subtitle?: string;
  fields: FieldConfig[];
  columns?: 1 | 2;
  backgroundColor?: string;
}

// Detail view item configuration
export interface DetailItem {
  label: string;
  value: (entity: any) => ReactNode;
  colSpan?: 1 | 2;
}

// Detail view section configuration
export interface DetailSection {
  title: string;
  titleColor?: string;
  items: DetailItem[];
  columns?: 1 | 2;
}

// API configuration
export interface ApiConfig {
  basePath: string;
  endpoints?: {
    list?: string;
    get?: string;
    create?: string;
    update?: string;
    delete?: string;
  };
}

// List item configuration
export interface ListItemConfig {
  title: (entity: any) => string;
  subtitle?: (entity: any) => string;
  description?: (entity: any) => string;
  badges?: Array<{
    field?: string;
    label: string | ((entity: any) => string);
    color: string;
    showWhen?: (entity: any) => boolean;
  }>;
  avatar?: {
    text: (entity: any) => string;
    backgroundColor?: string;
  };
}

// Filter configuration
export interface FilterConfig {
  id: string;
  label: string;
  type: 'select' | 'text' | 'date';
  options?: Array<{ value: string; label: string }> | 'dynamic';
  loadOptions?: () => Promise<Array<{ value: string; label: string }>>;
}

// Entity configuration
export interface EntityConfig<T = any> {
  // Basic info
  name: {
    singular: string;
    plural: string;
  };
  
  // Theme
  theme: DatabaseTheme;
  
  // API configuration
  api: ApiConfig;
  
  // Form configuration
  form: {
    sections: SectionConfig[];
    // Transform form data before submission
    transformBeforeSubmit?: (data: Partial<T>) => Partial<T>;
    // Initial data for new entities
    defaultValues?: Partial<T>;
  };
  
  // Detail view configuration
  detail: {
    header?: {
      title: (entity: T) => string;
      subtitle?: (entity: T) => string;
      avatar?: {
        text: (entity: T) => string;
        size?: 'sm' | 'md' | 'lg';
      };
      badges?: Array<{
        label: string | ((entity: T) => string);
        color: string;
        showWhen: (entity: T) => boolean;
      }>;
    };
    sections: DetailSection[];
    actions?: {
      edit?: boolean;
      delete?: boolean;
      custom?: Array<{
        label: string;
        onClick: (entity: T) => void | Promise<void>;
        color?: string;
      }>;
    };
  };
  
  // List configuration
  list: {
    title: string;
    description: string;
    searchPlaceholder: string;
    filters?: FilterConfig[];
    item: ListItemConfig;
    // Enable/disable features
    features?: {
      create?: boolean;
      search?: boolean;
      pagination?: boolean;
      filters?: boolean;
    };
    // Search configuration
    searchStrategy?: 'frontend' | 'backend'; // Default: 'frontend'
    searchableFields?: string[]; // Fields to search in frontend mode
    minSearchLength?: number; // Minimum characters before searching (default: 0)
    // Optional info section to display before the list
    infoSection?: {
      title: string;
      content: string;
      icon?: ReactNode;
    };
  };
  
  // Service configuration
  service?: {
    // Map API responses to frontend models
    mapResponse?: (data: any) => T;
    // Map frontend models to API requests
    mapRequest?: (data: Partial<T>) => any;
    // Custom CRUD methods
    create?: (data: Partial<T>, token?: string) => Promise<T>;
    update?: (id: string, data: Partial<T>, token?: string) => Promise<T>;
    delete?: (id: string, token?: string) => Promise<void>;
    // Custom service methods
    customMethods?: Record<string, (id?: string, data?: any) => Promise<any>>;
  };
  
  // Custom hooks for business logic
  hooks?: {
    beforeCreate?: (data: Partial<T>) => Promise<Partial<T>>;
    afterCreate?: (entity: T) => Promise<void>;
    beforeUpdate?: (id: string, data: Partial<T>) => Promise<Partial<T>>;
    afterUpdate?: (entity: T) => Promise<void>;
    beforeDelete?: (id: string) => Promise<boolean>;
    afterDelete?: (id: string) => Promise<void>;
  };
  
  // Labels and messages
  labels?: {
    createButton?: string;
    createModalTitle?: string;
    editModalTitle?: string;
    detailModalTitle?: string;
    deleteConfirmation?: string;
    emptyState?: string;
  };
}

// Type helper for creating entity configs with proper typing
export function defineEntityConfig<T>(config: EntityConfig<T>): EntityConfig<T> {
  return config;
}

// Response types for pagination
export interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}

// Generic CRUD service interface
export interface CrudService<T> {
  getList(filters?: Record<string, any>): Promise<PaginatedResponse<T>>;
  getOne(id: string): Promise<T>;
  create(data: Partial<T>): Promise<T>;
  update(id: string, data: Partial<T>): Promise<T>;
  delete(id: string): Promise<void>;
}