// Database Entity Configuration Types

import type { ReactNode } from "react";
import type { MotoConceptKey } from "~/lib/moto-concepts";

// Field types supported by the database forms
type FieldType =
  | "text"
  | "email"
  | "password"
  | "textarea"
  | "select"
  | "multiselect"
  | "checkbox"
  | "custom"
  | "number"
  | "date";

// Base field configuration - extends the form field type
interface FieldConfig {
  name: string;
  label: string;
  type: FieldType;
  required?: boolean;
  placeholder?: string;
  helperText?: string;
  description?: string;
  validation?: (value: unknown) => string | null;
  // For select/multiselect fields - supports both sync and async options
  options?:
    | Array<{ value: string; label: string }>
    | (() => Promise<Array<{ value: string; label: string }>>);
  loadOptions?: () => Promise<Array<{ value: string; label: string }>>;
  // For custom fields
  component?: React.ComponentType<{
    value: unknown;
    onChange: (value: unknown) => void;
    label: string;
    required?: boolean;
    includeEmpty?: boolean;
    emptyLabel?: string;
  }>;
  /**
   * Extra props merged into `component` before the form's own props. Keeps a
   * shared custom field reusable with different presets (e.g. a per-room
   * default colour) without wrapping it in a second component.
   */
  componentProps?: Record<string, unknown>;
  // Grid layout
  colSpan?: 1 | 2;
  autoComplete?: string;
  // For number fields
  min?: number;
  max?: number;
  // For text/email/textarea fields
  maxLength?: number;
}

// Form field type (imported from database-form.tsx)
interface FormField {
  name: string;
  label: string;
  type:
    | "text"
    | "email"
    | "select"
    | "multiselect"
    | "textarea"
    | "password"
    | "checkbox"
    | "custom"
    | "number"
    | "date";
  required?: boolean;
  placeholder?: string;
  options?:
    | Array<{ value: string; label: string }>
    | (() => Promise<Array<{ value: string; label: string }>>);
  validation?: (value: unknown) => string | null;
  component?: React.ComponentType<{
    value: unknown;
    onChange: (value: unknown) => void;
    label: string;
    required?: boolean;
    includeEmpty?: boolean;
    emptyLabel?: string;
  }>;
  /**
   * Extra props merged into `component` before the form's own props. Keeps a
   * shared custom field reusable with different presets (e.g. a per-room
   * default colour) without wrapping it in a second component.
   */
  componentProps?: Record<string, unknown>;
  helperText?: string;
  autoComplete?: string;
  colSpan?: 1 | 2;
  min?: number;
  max?: number;
  maxLength?: number;
  /** When true, the field is rendered as read-only and cannot be edited. */
  disabled?: boolean;
}

// Form section type (imported from database-form.tsx)
interface FormSection {
  title: string;
  subtitle?: string;
  fields: FormField[];
  columns?: 1 | 2;
  /** Optional concept driving the section's header icon (gray tile, MotoConceptIcon). */
  concept?: MotoConceptKey;
}

// Section configuration for forms
export interface SectionConfig {
  title: string;
  subtitle?: string;
  fields: FieldConfig[];
  columns?: 1 | 2;
  /** Optional concept driving the section's header icon (gray tile, MotoConceptIcon). */
  concept?: MotoConceptKey;
}

// Detail view item configuration
interface DetailItem<T = Record<string, unknown>> {
  label: string;
  value: (entity: T) => ReactNode;
  colSpan?: 1 | 2;
}

// Detail view section configuration
interface DetailSection<T = Record<string, unknown>> {
  title: string;
  items: DetailItem<T>[];
  columns?: 1 | 2;
}

// API configuration
interface ApiConfig {
  basePath: string;
  listParams?: Record<string, string>;
  endpoints?: {
    list?: string;
    get?: string;
    create?: string;
    update?: string;
    delete?: string;
  };
}

// List item configuration
interface ListItemConfig<T = Record<string, unknown>> {
  title: (entity: T) => string;
  subtitle?: (entity: T) => string;
  description?: (entity: T) => string;
  avatar?: {
    text: (entity: T) => string;
  };
}

// Filter configuration
interface FilterConfig {
  id: string;
  label: string;
  type: "select" | "text" | "date";
  options?:
    | Array<{ value: string; label: string }>
    | "dynamic"
    | (() => Promise<Array<{ value: string; label: string }>>);
  loadOptions?: () => Promise<Array<{ value: string; label: string }>>;
  placeholder?: string;
}

// Entity configuration
export interface EntityConfig<T = Record<string, unknown>> {
  // Basic info
  name: {
    singular: string;
    plural: string;
  };

  // Concept driving the entity's icon/tone across the database area (see
  // MOTO_CONCEPTS). Replaces the former hand-rolled per-entity theme object.
  concept: MotoConceptKey;

  // Navigation
  backUrl?: string;

  // API configuration
  api: ApiConfig;

  // Form configuration
  form: {
    sections: SectionConfig[];
    // Transform form data before submission
    transformBeforeSubmit?: (data: Partial<T>) => Partial<T>;
    // Initial data for new entities
    defaultValues?: Partial<T>;
    // Form validation
    validation?: (
      data: Record<string, unknown>,
    ) => Record<string, string> | null;
  };

  // Detail view configuration
  detail: {
    header?: {
      title: (entity: T) => string;
      subtitle?: (entity: T) => string;
      avatar?: {
        text: (entity: T) => string;
        size?: "sm" | "md" | "lg";
      };
    };
    sections: DetailSection<T>[];
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
    item: ListItemConfig<T>;
    // Enable/disable features
    features?: {
      create?: boolean;
      search?: boolean;
      pagination?: boolean;
      filters?: boolean;
    };
    // Search configuration
    searchStrategy?: "frontend" | "backend"; // Default: 'frontend'
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
    mapResponse?: (data: unknown) => T;
    // Map frontend models to API requests
    mapRequest?: (data: Partial<T>) => Record<string, unknown>;
    // Custom CRUD methods
    create?: (data: Partial<T>, token?: string) => Promise<T>;
    update?: (id: string, data: Partial<T>, token?: string) => Promise<T>;
    delete?: (id: string, token?: string) => Promise<void>;
    // Custom service methods
    customMethods?: Record<
      string,
      (id?: string, data?: Record<string, unknown>) => Promise<unknown>
    >;
    // Get one method with explicit typing
    getOne?: (id?: string, data?: Record<string, unknown>) => Promise<unknown>;
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

  // Labels and messages. The modal titles are optional because not every
  // entity renders DatabaseFormModal; the modal itself fails loudly when the
  // title for its mode is missing.
  labels: {
    createButton?: string;
    createModalTitle?: string;
    editModalTitle?: string;
    detailModalTitle?: string;
    deleteConfirmation?: string;
    emptyState?: string;
    empty?: {
      title?: string;
      description?: string;
    };
  };

  // Optional callback after successful creation
  onCreateSuccess?: (result: T) => unknown;
}

// Type helper for creating entity configs with proper typing
export function defineEntityConfig<T>(
  config: EntityConfig<T>,
): EntityConfig<T> {
  return config;
}

// Utility to convert SectionConfig to FormSection
export function configToFormSection(section: SectionConfig): FormSection {
  return {
    title: section.title,
    subtitle: section.subtitle,
    fields: section.fields.map((field) => ({
      name: field.name,
      label: field.label,
      type: field.type,
      required: field.required,
      placeholder: field.placeholder,
      options: field.options,
      validation: field.validation,
      component: field.component,
      componentProps: field.componentProps,
      helperText: field.helperText,
      autoComplete: field.autoComplete,
      colSpan: field.colSpan,
      min: field.min,
      max: field.max,
      maxLength: field.maxLength,
    })),
    columns: section.columns,
    concept: section.concept,
  };
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
  getList(filters?: Record<string, unknown>): Promise<PaginatedResponse<T>>;
  getOne(id: string): Promise<T>;
  create(data: Partial<T>): Promise<T>;
  update(id: string, data: Partial<T>): Promise<T>;
  /** Returns null on success, or a user-facing error message string on failure. */
  delete(id: string): Promise<string | null>;
}
