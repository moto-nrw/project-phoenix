"use client";

import { useState, useEffect, useCallback } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DatabaseSelect" });

interface SelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
}

interface DatabaseSelectProps {
  // Core props
  readonly id?: string;
  readonly name: string;
  readonly label?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;

  // Options - either static or async
  readonly options?: ReadonlyArray<SelectOption>;
  readonly loadOptions?: () => Promise<ReadonlyArray<SelectOption>>;

  // UI props
  readonly placeholder?: string;
  readonly emptyOptionLabel?: string;
  readonly required?: boolean;
  readonly disabled?: boolean;
  readonly loading?: boolean;
  readonly error?: string;
  readonly helperText?: string;
  readonly className?: string;
  readonly includeEmpty?: boolean;
}

export function DatabaseSelect({
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
}: DatabaseSelectProps) {
  const [options, setOptions] = useState<readonly SelectOption[]>(
    staticOptions ?? [],
  );
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
        logger.error("failed to load options", {
          error: err instanceof Error ? err.message : String(err),
        });
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

  const isLoading = loading || externalLoading;
  const displayError = error ?? externalError;

  const selectOptions = [
    ...(includeEmpty
      ? [
          {
            value: "",
            label: isLoading ? "Lädt..." : (emptyOptionLabel ?? placeholder),
          },
        ]
      : []),
    ...options.map((option) => ({
      value: option.value,
      label: option.label,
      disabled: option.disabled,
    })),
    ...(!isLoading && options.length === 0 && !includeEmpty
      ? [{ value: "", label: "Keine Optionen verfügbar", disabled: true }]
      : []),
  ];

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

      <CustomSelect
        ariaLabel={label ?? name}
        id={id ?? name}
        name={name}
        value={value}
        options={selectOptions}
        onChange={onChange}
        disabled={disabled || isLoading}
        required={required}
        invalid={Boolean(displayError)}
        placeholder={isLoading ? "Lädt..." : (emptyOptionLabel ?? placeholder)}
        className={`${isLoading ? "cursor-wait opacity-50" : ""} ${className}`}
      />

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
interface EntitySelectProps extends Omit<
  DatabaseSelectProps,
  "loadOptions" | "options"
> {
  readonly entityType: "groups" | "rooms" | "teachers" | "activities";
  readonly filters?: Record<string, unknown>;
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

    const queryString = params.toString();
    const url = queryString
      ? `/api/${entityType}?${queryString}`
      : `/api/${entityType}`;
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

export function GroupSelect(
  props: Readonly<Omit<EntitySelectProps, "entityType">>,
) {
  return (
    <EntitySelect
      {...props}
      entityType="groups"
      label={props.label ?? "Gruppe"}
    />
  );
}
