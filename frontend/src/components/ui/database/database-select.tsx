"use client";

import { useState, useEffect, useCallback } from "react";

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface DatabaseSelectProps {
  // Core props
  id?: string;
  name: string;
  label?: string;
  value: string;
  onChange: (value: string) => void;

  // Options - either static or async
  options?: SelectOption[];
  loadOptions?: () => Promise<SelectOption[]>;

  // UI props
  placeholder?: string;
  emptyOptionLabel?: string;
  required?: boolean;
  disabled?: boolean;
  loading?: boolean;
  error?: string;
  helperText?: string;
  className?: string;
  includeEmpty?: boolean;

  // For theme-aware styling
  focusRingColor?: string;
}

function DatabaseSelect({
  id,
  name,
  label,
  value,
  onChange,
  options: staticOptions,
  loadOptions,
  placeholder = "Bitte wählen",
  emptyOptionLabel,
  required = false,
  disabled = false,
  loading: externalLoading = false,
  error: externalError,
  helperText,
  className = "",
  includeEmpty = true,
  focusRingColor = "focus:ring-blue-500",
}: DatabaseSelectProps) {
  const [options, setOptions] = useState<SelectOption[]>(staticOptions ?? []);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load async options if loadOptions is provided
  useEffect(() => {
    if (!loadOptions || staticOptions) return;

    const fetchOptions = async () => {
      try {
        setLoading(true);
        setError(null);
        const loadedOptions = await loadOptions();
        setOptions(loadedOptions);
      } catch (err) {
        console.error("Error loading options:", err);
        setError("Fehler beim Laden der Optionen");
        setOptions([]);
      } finally {
        setLoading(false);
      }
    };

    void fetchOptions();
  }, [loadOptions, staticOptions]);

  // Update options if staticOptions change
  useEffect(() => {
    if (staticOptions) {
      setOptions(staticOptions);
    }
  }, [staticOptions]);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLSelectElement>) => {
      onChange(e.target.value);
    },
    [onChange],
  );

  const isLoading = loading || externalLoading;
  const displayError = error ?? externalError;

  // Base input classes matching StudentForm styling
  const baseClasses = `
    w-full rounded-lg border border-gray-300 px-3 py-2 md:px-4 
    text-sm md:text-base transition-all duration-200 focus:ring-2 ${focusRingColor} focus:outline-none
    ${isLoading ? "opacity-50 cursor-wait" : ""}
    ${disabled ? "bg-gray-50 cursor-not-allowed" : ""}
    ${displayError ? "border-red-300 focus:ring-red-500" : ""}
    ${className}
  `.trim();

  return (
    <div className="w-full">
      {label && (
        <label
          htmlFor={id ?? name}
          className="mb-1 block text-xs font-medium text-gray-700 md:text-sm"
        >
          {label}
          {required && "*"}
        </label>
      )}

      <select
        id={id ?? name}
        name={name}
        value={value}
        onChange={handleChange}
        disabled={disabled || isLoading}
        required={required}
        className={baseClasses}
      >
        {/* Empty option */}
        {includeEmpty && (
          <option value="">
            {isLoading ? "Lädt..." : (emptyOptionLabel ?? placeholder)}
          </option>
        )}

        {/* Render options */}
        {options.map((option) => (
          <option
            key={option.value}
            value={option.value}
            disabled={option.disabled}
          >
            {option.label}
          </option>
        ))}

        {/* Show message if no options available */}
        {!isLoading && options.length === 0 && !includeEmpty && (
          <option value="" disabled>
            Keine Optionen verfügbar
          </option>
        )}
      </select>

      {/* Helper text or error message */}
      {displayError && (
        <p className="mt-1 text-xs text-red-600 md:text-sm">{displayError}</p>
      )}
      {!displayError && helperText && (
        <p className="mt-1 text-xs text-gray-500 md:text-sm">{helperText}</p>
      )}
    </div>
  );
}

/**
 * Specialized select components using DatabaseSelect as base
 */

// Example: EntitySelect for loading entities from API
interface EntitySelectProps
  extends Omit<DatabaseSelectProps, "loadOptions" | "options"> {
  entityType: "groups" | "rooms" | "teachers" | "activities";
  filters?: Record<string, unknown>;
}

function EntitySelect({ entityType, filters, ...props }: EntitySelectProps) {
  const loadOptions = useCallback(async () => {
    const params = new URLSearchParams();
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value !== null && value !== undefined) {
          // Convert value to string safely
          let stringValue: string;
          if (typeof value === "string") {
            stringValue = value;
          } else if (typeof value === "number" || typeof value === "boolean") {
            stringValue = String(value);
          } else {
            // For objects and other types, use JSON.stringify
            stringValue = JSON.stringify(value);
          }
          params.append(key, stringValue);
        }
      });
    }

    const url = `/api/${entityType}${params.toString() ? `?${params}` : ""}`;
    const response = await fetch(url);

    if (!response.ok) {
      throw new Error(`Failed to load ${entityType}`);
    }

    const data = (await response.json()) as
      | Array<{ id: string | number; name: string }>
      | { data: Array<{ id: string | number; name: string }> };

    // Handle different response formats
    const items = Array.isArray(data) ? data : data.data;

    return items.map((item) => ({
      value: String(item.id),
      label: item.name,
    }));
  }, [entityType, filters]);

  return <DatabaseSelect {...props} loadOptions={loadOptions} />;
}

/**
 * Convenience components for specific entity types
 */

export function GroupSelect(props: Omit<EntitySelectProps, "entityType">) {
  return (
    <EntitySelect
      {...props}
      entityType="groups"
      label={props.label ?? "Gruppe"}
    />
  );
}
