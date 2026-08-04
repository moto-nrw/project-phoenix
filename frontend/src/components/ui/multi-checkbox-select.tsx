"use client";

import { ChevronDown } from "lucide-react";
import { useCallback, useId, useRef, useState } from "react";

import { Checkbox } from "~/components/ui/checkbox";
import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { cn } from "~/lib/utils";

interface MultiCheckboxSelectOption {
  readonly value: string;
  readonly label: string;
  readonly badge?: string;
  readonly disabled?: boolean;
}

interface MultiCheckboxSelectProps {
  readonly value: readonly string[];
  readonly options: readonly MultiCheckboxSelectOption[];
  readonly onChange: (value: string[]) => void;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly emptyLabel?: string;
  readonly unavailableLabel?: string;
  readonly multipleLabel?: (count: number) => string;
  readonly disabled?: boolean;
  readonly className?: string;
  readonly menuClassName?: string;
  /**
   * Adds a filter input and a "Alle auswählen / Aufheben" row to the menu —
   * for long option lists (e.g. all AGs of a big school). Select-all and
   * clear act on the currently FILTERED options only.
   */
  readonly searchable?: boolean;
  readonly searchPlaceholder?: string;
}

const TRIGGER_CLASS =
  "moto-content-surface flex h-10 w-full items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const defaultMultipleLabel = (count: number) => `${count} ausgewählt`;

export function MultiCheckboxSelect({
  value,
  options,
  onChange,
  id,
  ariaLabel,
  emptyLabel = "Keine Auswahl",
  unavailableLabel = "Keine Optionen verfügbar",
  multipleLabel = defaultMultipleLabel,
  disabled = false,
  className = "",
  menuClassName = "",
  searchable = false,
  searchPlaceholder = "Suchen...",
}: MultiCheckboxSelectProps) {
  const generatedId = useId();
  const triggerId = id ?? generatedId;
  const menuId = `${triggerId}-menu`;
  const containerRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [open, setOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const selectedValues = new Set(value);
  const selectedOptions = options.filter((option) =>
    selectedValues.has(option.value),
  );
  const isDisabled = disabled || options.length === 0;
  const triggerText =
    options.length === 0
      ? unavailableLabel
      : selectedOptions.length === 0
        ? emptyLabel
        : selectedOptions.length === 1
          ? selectedOptions[0]?.label
          : multipleLabel(selectedOptions.length);

  const closeMenu = useCallback(() => {
    setOpen(false);
    setSearchTerm("");
  }, []);
  useClickOutside(containerRef, closeMenu, open);

  const toggleValue = useCallback(
    (optionValue: string) => {
      const next = new Set(value);
      if (next.has(optionValue)) {
        next.delete(optionValue);
      } else {
        next.add(optionValue);
      }
      onChange(Array.from(next));
    },
    [onChange, value],
  );

  const normalizedTerm = searchTerm.trim().toLowerCase();
  const visibleOptions =
    searchable && normalizedTerm
      ? options.filter((option) =>
          option.label.toLowerCase().includes(normalizedTerm),
        )
      : options;

  const selectableVisible = visibleOptions.filter((o) => !o.disabled);
  const allVisibleSelected =
    selectableVisible.length > 0 &&
    selectableVisible.every((o) => selectedValues.has(o.value));

  const selectAllVisible = useCallback(() => {
    const next = new Set(value);
    for (const option of selectableVisible) next.add(option.value);
    onChange(Array.from(next));
  }, [onChange, value, selectableVisible]);

  const clearVisible = useCallback(() => {
    const visible = new Set(selectableVisible.map((o) => o.value));
    onChange(value.filter((v) => !visible.has(v)));
  }, [onChange, value, selectableVisible]);

  const toggleOpen = useCallback(() => {
    if (open) setSearchTerm("");
    setOpen(!open);
  }, [open]);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLElement>) => {
      if (!open || event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      closeMenu();
      triggerRef.current?.focus();
    },
    [closeMenu, open],
  );

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        id={triggerId}
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={isDisabled}
        onClick={toggleOpen}
        onKeyDown={handleKeyDown}
        className={cn(TRIGGER_CLASS, className)}
      >
        <span
          className={cn(
            "min-w-0 flex-1 truncate",
            selectedOptions.length > 0 ? "text-gray-900" : "text-gray-500",
          )}
        >
          {triggerText}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "h-4 w-4 flex-shrink-0 text-gray-400 transition-transform",
            open ? "rotate-180" : "",
          )}
        />
      </button>

      {open ? (
        <div
          id={menuId}
          role="dialog"
          aria-label={ariaLabel}
          onKeyDown={handleKeyDown}
          className={cn(
            "absolute top-full left-0 z-50 mt-1 min-w-full overflow-hidden rounded-xl border border-gray-200 bg-white py-2 shadow-lg",
            menuClassName,
          )}
        >
          <div className="max-h-[17rem] overflow-y-auto overscroll-contain">
            {searchable && (
              <div className="sticky top-0 z-10 space-y-1.5 border-b border-gray-100 bg-white px-3 pb-2">
                <input
                  type="text"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  placeholder={searchPlaceholder}
                  aria-label={searchPlaceholder}
                  className="block w-full rounded-lg border-0 bg-white px-3 py-1.5 text-sm text-gray-900 ring-1 ring-gray-200 transition-all ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
                />
                {selectableVisible.length > 0 && (
                  <button
                    type="button"
                    onClick={
                      allVisibleSelected ? clearVisible : selectAllVisible
                    }
                    className="text-xs font-medium text-gray-600 underline underline-offset-2 hover:text-gray-900"
                  >
                    {allVisibleSelected
                      ? normalizedTerm
                        ? "Treffer abwählen"
                        : "Auswahl aufheben"
                      : normalizedTerm
                        ? "Alle Treffer auswählen"
                        : "Alle auswählen"}
                  </button>
                )}
              </div>
            )}
            {searchable && visibleOptions.length === 0 && (
              <p className="px-4 py-2.5 text-sm text-gray-500">
                Keine Treffer.
              </p>
            )}
            {visibleOptions.map((option) => (
              <label
                key={option.value}
                className={cn(
                  "flex w-full cursor-pointer items-start gap-3 px-4 py-2.5 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50",
                  option.disabled
                    ? "cursor-not-allowed text-gray-400"
                    : "text-gray-700",
                )}
              >
                <Checkbox
                  checked={selectedValues.has(option.value)}
                  disabled={option.disabled}
                  onChange={() => toggleValue(option.value)}
                  className="mt-0.5"
                />
                <span className="min-w-0 flex-1">
                  <span className="block font-medium break-words">
                    {option.label}
                  </span>
                  {option.badge ? (
                    <span className="mt-1 inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                      {option.badge}
                    </span>
                  ) : null}
                </span>
              </label>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
