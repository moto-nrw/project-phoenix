"use client";

import { useState, useEffect } from "react";
import type { DatabaseTheme } from "./themes";
import { getThemeClassNames } from "./themes";

export interface FormField {
  name: string;
  label: string;
  type: 'text' | 'email' | 'select' | 'textarea' | 'password' | 'checkbox' | 'custom';
  required?: boolean;
  placeholder?: string;
  options?: Array<{ value: string; label: string }>;
  loadOptions?: () => Promise<Array<{ value: string; label: string }>>;
  validation?: (value: unknown) => string | null;
  component?: React.ComponentType<{
    value: unknown;
    onChange: (value: unknown) => void;
    label: string;
    required?: boolean;
    includeEmpty?: boolean;
    emptyLabel?: string;
  }>;
  helperText?: string;
  autoComplete?: string;
  colSpan?: 1 | 2;
}

export interface FormSection {
  title: string;
  subtitle?: string;
  fields: FormField[];
  columns?: 1 | 2;
  backgroundColor?: string; // Override theme background
}

export interface DatabaseFormProps<T = Record<string, unknown>> {
  theme: DatabaseTheme;
  sections: FormSection[];
  onSubmit: (data: T) => Promise<void>;
  onCancel: () => void;
  initialData?: Partial<T>;
  isLoading?: boolean;
  error?: string | null;
  submitLabel: string;
  submitButtonGradient?: string; // Override default gradient
}

export function DatabaseForm<T = Record<string, unknown>>({
  theme,
  sections,
  onSubmit,
  onCancel,
  initialData,
  isLoading,
  error: externalError,
  submitLabel,
  submitButtonGradient,
}: DatabaseFormProps<T>) {
  const [formData, setFormData] = useState<Record<string, unknown>>({});
  const [error, setError] = useState<string | null>(null);
  const [asyncOptions, setAsyncOptions] = useState<Record<string, Array<{ value: string; label: string }>>>({});
  const [loadingOptions, setLoadingOptions] = useState<Record<string, boolean>>({});
  const themeClasses = getThemeClassNames(theme);

  // Initialize form data from sections
  useEffect(() => {
    const initialFormData: Record<string, unknown> = {};
    
    // Set defaults from sections
    sections.forEach(section => {
      section.fields.forEach(field => {
        if (field.type === 'checkbox') {
          initialFormData[field.name] = false;
        } else {
          initialFormData[field.name] = '';
        }
      });
    });

    // Override with initial data if provided
    if (initialData) {
      Object.keys(initialData).forEach(key => {
        if (initialData[key as keyof T] !== undefined) {
          initialFormData[key] = initialData[key as keyof T];
        }
      });
    }

    setFormData(initialFormData);
  }, [initialData, sections]);

  // Load async options for select fields
  useEffect(() => {
    const loadAsyncOptions = async () => {
      for (const section of sections) {
        for (const field of section.fields) {
          if (field.type === 'select' && field.loadOptions) {
            setLoadingOptions(prev => ({ ...prev, [field.name]: true }));
            try {
              const options = await field.loadOptions();
              setAsyncOptions(prev => ({ ...prev, [field.name]: options }));
            } catch (error) {
              console.error(`Error loading options for ${field.name}:`, error);
              setAsyncOptions(prev => ({ ...prev, [field.name]: [] }));
            } finally {
              setLoadingOptions(prev => ({ ...prev, [field.name]: false }));
            }
          }
        }
      }
    };
    
    void loadAsyncOptions();
  }, [sections]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>
  ) => {
    const { name, value, type } = e.target as HTMLInputElement;

    if (type === 'checkbox') {
      const { checked } = e.target as HTMLInputElement;
      setFormData(prev => ({
        ...prev,
        [name]: checked,
      }));
    } else {
      setFormData(prev => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validate required fields
    for (const section of sections) {
      for (const field of section.fields) {
        const value = formData[field.name];
        if (field.required && (!value || (typeof value === 'string' && !value.trim()))) {
          setError(`${field.label} ist erforderlich.`);
          return;
        }
        
        // Custom validation
        if (field.validation) {
          const validationError = field.validation(formData[field.name]);
          if (validationError) {
            setError(validationError);
            return;
          }
        }
      }
    }

    try {
      await onSubmit(formData as T);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        err instanceof Error 
          ? err.message 
          : "Fehler beim Speichern der Daten. Bitte versuchen Sie es später erneut."
      );
    }
  };

  const renderField = (field: FormField, sectionBackground: string) => {
    // Determine focus ring color based on section background
    const focusRingColor = sectionBackground.includes('blue') ? 'focus:ring-blue-500' 
      : sectionBackground.includes('purple') ? 'focus:ring-purple-500'
      : sectionBackground.includes('green') ? 'focus:ring-green-500'
      : sectionBackground.includes('orange') ? 'focus:ring-orange-500'
      : 'focus:ring-indigo-500';

    const baseInputClasses = `w-full rounded-lg border border-gray-300 px-3 py-2 md:px-4 md:py-2 text-sm md:text-base transition-all duration-200 focus:ring-2 ${focusRingColor} focus:outline-none`;

    switch (field.type) {
      case 'custom':
        if (!field.component) return null;
        const Component = field.component;
        return (
          <Component
            value={formData[field.name]}
            onChange={(value: unknown) => {
              setFormData(prev => ({
                ...prev,
                [field.name]: value,
              }));
            }}
            label={field.label}
            required={field.required}
            includeEmpty={true}
            emptyLabel={field.placeholder}
          />
        );

      case 'checkbox':
        return (
          <div className="flex items-center">
            <input
              type="checkbox"
              id={field.name}
              name={field.name}
              checked={Boolean(formData[field.name])}
              onChange={handleChange}
              className={`h-4 w-4 rounded border-gray-300 text-${theme.accent}-600 focus:ring-${theme.accent}-500`}
            />
            <label
              htmlFor={field.name}
              className="ml-2 block text-xs md:text-sm text-gray-700"
            >
              {field.label}
            </label>
            {field.helperText && (
              <p className="ml-2 text-xs md:text-sm text-gray-500">
                {field.helperText}
              </p>
            )}
          </div>
        );

      case 'textarea':
        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1 block text-xs md:text-sm font-medium text-gray-700"
            >
              {field.label}{field.required && '*'}
            </label>
            <textarea
              id={field.name}
              name={field.name}
              value={(formData[field.name] as string) ?? ''}
              onChange={handleChange}
              required={field.required}
              placeholder={field.placeholder}
              rows={3}
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs md:text-sm text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      case 'select':
        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1 block text-xs md:text-sm font-medium text-gray-700"
            >
              {field.label}{field.required && '*'}
            </label>
            <select
              id={field.name}
              name={field.name}
              value={(formData[field.name] as string) ?? ''}
              onChange={handleChange}
              required={field.required}
              className={baseInputClasses}
              disabled={loadingOptions[field.name]}
            >
              <option value="">
                {loadingOptions[field.name] 
                  ? 'Optionen werden geladen...' 
                  : (field.placeholder ?? 'Bitte wählen')}
              </option>
              {(field.options ?? asyncOptions[field.name] ?? []).map(option => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            {field.helperText && (
              <p className="mt-1 text-xs md:text-sm text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      default:
        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1 block text-xs md:text-sm font-medium text-gray-700"
            >
              {field.label}{field.required && '*'}
            </label>
            <input
              type={field.type}
              id={field.name}
              name={field.name}
              value={(formData[field.name] as string) ?? ''}
              onChange={handleChange}
              required={field.required}
              placeholder={field.placeholder}
              autoComplete={field.autoComplete}
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs md:text-sm text-gray-500">{field.helperText}</p>
            )}
          </div>
        );
    }
  };

  // Determine button gradient
  const buttonGradient = submitButtonGradient ?? `from-${theme.primary} to-${theme.secondary}`;
  const buttonHoverGradient = submitButtonGradient 
    ? submitButtonGradient.replace('500', '600').replace('600', '700')
    : `from-${theme.primary.replace('500', '600')} to-${theme.secondary.replace('600', '700')}`;

  return (
    <div className="overflow-hidden rounded-lg bg-white shadow-md">
      <div className="p-4 md:p-6">
        {(error ?? externalError) && (
          <div className="mb-4 md:mb-6 rounded-lg bg-red-50 p-3 md:p-4 text-sm md:text-base text-red-800">
            <p>{error ?? externalError}</p>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          {sections.map((section, sectionIndex) => {
            // Use custom background or theme background
            const bgClass = section.backgroundColor ?? themeClasses.background;
            const textClass = themeClasses.text;

            return (
              <div key={`section-${sectionIndex}`} className={`mb-6 md:mb-8 rounded-lg ${bgClass} p-3 md:p-4`}>
                <h2 className={`mb-3 md:mb-4 text-base md:text-lg font-medium ${textClass}`}>
                  {section.title}
                </h2>
                {section.subtitle && (
                  <p className="mb-3 md:mb-4 text-xs md:text-sm text-gray-600">{section.subtitle}</p>
                )}
                <div className={`grid grid-cols-1 gap-4 ${section.columns === 2 ? 'md:grid-cols-2' : ''}`}>
                  {section.fields.map((field) => (
                    <div 
                      key={field.name}
                      className={field.colSpan === 2 && section.columns === 2 ? 'md:col-span-2' : ''}
                    >
                      {renderField(field, bgClass)}
                    </div>
                  ))}
                </div>
              </div>
            );
          })}

          {/* Form actions - matching StudentForm exactly */}
          <div className="flex justify-end pt-4">
            <button
              type="button"
              onClick={onCancel}
              className="mr-2 rounded-lg px-3 py-2 md:px-4 text-sm md:text-base text-gray-700 shadow-sm transition-colors hover:bg-gray-100"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className={`rounded-lg bg-gradient-to-r ${buttonGradient} px-4 py-2 md:px-6 text-sm md:text-base text-white transition-all duration-200 hover:${buttonHoverGradient} hover:shadow-lg`}
              disabled={isLoading}
            >
              {isLoading ? "Wird gespeichert..." : submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}