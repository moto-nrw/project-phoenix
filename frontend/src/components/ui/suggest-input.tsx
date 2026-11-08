"use client";

import {
  type InputHTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useId,
  useMemo,
  useState,
} from "react";

import { foldForSearch } from "~/lib/search-fold";
import { cn } from "~/lib/utils";

import { AnchoredPopover } from "./anchored-popover";
import { FieldLabel, isInvalidAria, isRequiredField } from "./field-label";
import { Input } from "./input";

// In a dialog or slide-over the list is portaled into its focus scope: a list
// at the end of the body would sit outside the dialog's focus trap.
const SUGGEST_PORTAL_SCOPE =
  '[data-modal-focus-scope="true"], [data-date-picker-focus-trap="true"]';

const OPTION_CLASS =
  "flex w-full cursor-pointer items-center px-4 py-2 text-left text-sm transition-colors pointer-coarse:min-h-11";
const OPTION_IDLE_CLASS = "text-gray-700 hover:bg-gray-50";
const OPTION_ACTIVE_CLASS = "bg-gray-50 font-medium text-gray-900";

type NativeInputProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  | "value"
  | "defaultValue"
  | "onChange"
  | "role"
  | "list"
  | "type"
  | "autoComplete"
  | "aria-autocomplete"
  | "aria-expanded"
  | "aria-controls"
  | "aria-activedescendant"
>;

interface SuggestInputProps extends NativeInputProps {
  readonly value: string;
  readonly onValueChange: (value: string) => void;
  /** Known values to offer. Free text stays allowed. */
  readonly suggestions: readonly string[];
  /** Read out for the suggestion list, e.g. „Vorschläge für die Position“. */
  readonly listLabel: string;
  readonly label?: string;
  readonly hint?: string;
  readonly error?: string;
  readonly controlSize?: "default" | "compact";
  /** Symbol at the start of the field, e.g. a magnifier. */
  readonly leadingIcon?: ReactNode;
  /** Show at most this many matches. */
  readonly maxSuggestions?: number;
  /**
   * Marks the first match, so Enter takes it. Without it Enter keeps the typed
   * text and only an arrow key moves into the list.
   */
  readonly highlightFirst?: boolean;
  /** Called after a suggestion was taken over into the field. */
  readonly onSuggestionSelect?: (value: string) => void;
}

/**
 * Free text with suggestions from known values (combobox with a listbox).
 * Arrow keys move through the list, Enter takes the marked entry, Escape
 * closes only the list. Focus stays in the field the whole time.
 */
export function SuggestInput({
  value,
  onValueChange,
  suggestions,
  listLabel,
  label,
  hint,
  error,
  controlSize = "default",
  leadingIcon,
  maxSuggestions,
  highlightFirst = false,
  onSuggestionSelect,
  id,
  name,
  className,
  disabled,
  readOnly,
  onFocus,
  onBlur,
  onKeyDown,
  "aria-invalid": ariaInvalid,
  ...props
}: SuggestInputProps) {
  const fallbackId = useId();
  const inputId = id ?? name ?? fallbackId;
  const [open, setOpen] = useState(false);
  const firstIndex = highlightFirst ? 0 : -1;
  const [activeIndex, setActiveIndex] = useState(firstIndex);

  const matches = useMemo(() => {
    const query = foldForSearch(value.trim());
    const seen = new Set<string>();
    const found = suggestions.filter((suggestion) => {
      if (seen.has(suggestion)) return false;
      seen.add(suggestion);
      return !query || foldForSearch(suggestion).includes(query);
    });
    return maxSuggestions === undefined
      ? found
      : found.slice(0, maxSuggestions);
  }, [maxSuggestions, suggestions, value]);

  const listOpen = open && matches.length > 0 && !disabled && !readOnly;
  const optionId = (index: number) => `${inputId}-suggestion-${index}`;
  const activeOption =
    listOpen && activeIndex >= 0 && activeIndex < matches.length
      ? activeIndex
      : -1;

  const activeOptionId = activeOption >= 0 ? optionId(activeOption) : null;
  useEffect(() => {
    if (!activeOptionId) return;
    document
      .getElementById(activeOptionId)
      ?.scrollIntoView?.({ block: "nearest" });
  }, [activeOptionId]);

  const openList = () => {
    setOpen(true);
    setActiveIndex(firstIndex);
  };

  const closeList = () => {
    setOpen(false);
    setActiveIndex(firstIndex);
  };

  const take = (suggestion: string) => {
    onValueChange(suggestion);
    closeList();
    onSuggestionSelect?.(suggestion);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" && !listOpen && matches.length > 0) {
      event.preventDefault();
      setOpen(true);
      setActiveIndex(0);
      return;
    }
    if (listOpen) {
      const last = matches.length - 1;
      switch (event.key) {
        case "Escape":
          // Close only the list. stopImmediatePropagation keeps the Escape
          // from the document listener of a surrounding dialog.
          event.preventDefault();
          event.nativeEvent.stopImmediatePropagation();
          closeList();
          return;
        case "ArrowDown":
          event.preventDefault();
          setActiveIndex(activeOption >= last ? 0 : activeOption + 1);
          return;
        case "ArrowUp":
          event.preventDefault();
          setActiveIndex(activeOption <= 0 ? last : activeOption - 1);
          return;
        case "Home":
          event.preventDefault();
          setActiveIndex(0);
          return;
        case "End":
          event.preventDefault();
          setActiveIndex(last);
          return;
        case "Enter": {
          const suggestion =
            activeOption >= 0 ? matches[activeOption] : undefined;
          if (suggestion !== undefined) {
            event.preventDefault();
            take(suggestion);
            return;
          }
          break;
        }
        default:
          break;
      }
    }
    onKeyDown?.(event);
  };

  const invalid = Boolean(error) || isInvalidAria(ariaInvalid);
  const compact = controlSize === "compact";

  return (
    <div>
      {label ? (
        <FieldLabel
          htmlFor={inputId}
          required={isRequiredField(props)}
          invalid={invalid}
        >
          {label}
        </FieldLabel>
      ) : null}
      <AnchoredPopover<HTMLInputElement>
        open={listOpen}
        onOpenChange={(next) => (next ? openList() : closeList())}
        ariaLabel={listLabel}
        panelRole="listbox"
        restoreFocusOnClose={false}
        portalScopeSelector={SUGGEST_PORTAL_SCOPE}
        className="py-1"
        renderTrigger={({ ref, panelId }) => (
          <div className="relative">
            {leadingIcon ? (
              <span
                aria-hidden="true"
                className={cn(
                  "pointer-events-none absolute top-0 left-3 z-10 flex items-center text-gray-400 [&>svg]:h-4 [&>svg]:w-4",
                  compact ? "h-10" : "h-12",
                )}
              >
                {leadingIcon}
              </span>
            ) : null}
            <Input
              {...props}
              ref={ref}
              id={inputId}
              name={name}
              type="text"
              hint={hint}
              error={error}
              aria-invalid={ariaInvalid}
              controlSize={controlSize}
              disabled={disabled}
              readOnly={readOnly}
              className={cn(
                leadingIcon && (compact ? "pl-9" : "pl-10"),
                className,
              )}
              value={value}
              autoComplete="off"
              role="combobox"
              aria-autocomplete="list"
              aria-expanded={listOpen}
              aria-controls={listOpen ? panelId : undefined}
              aria-activedescendant={activeOptionId ?? undefined}
              onChange={(event) => {
                onValueChange(event.target.value);
                openList();
              }}
              onFocus={(event) => {
                openList();
                onFocus?.(event);
              }}
              onBlur={(event) => {
                closeList();
                onBlur?.(event);
              }}
              onKeyDown={handleKeyDown}
            />
          </div>
        )}
      >
        {() => (
          <div role="presentation">
            {matches.map((suggestion, index) => (
              <div
                key={suggestion}
                id={optionId(index)}
                role="option"
                // Never focused: the field keeps the focus (aria-activedescendant).
                tabIndex={-1}
                aria-selected={index === activeOption}
                onMouseEnter={() => setActiveIndex(index)}
                // Take the entry on pointer-down and keep the focus in the
                // field. Safari/iOS does not focus a tapped element, so the
                // field would blur first and the list would close before a
                // click arrived.
                onPointerDown={(event) => {
                  event.preventDefault();
                  take(suggestion);
                }}
                className={cn(
                  OPTION_CLASS,
                  index === activeOption
                    ? OPTION_ACTIVE_CLASS
                    : OPTION_IDLE_CLASS,
                )}
              >
                <span className="min-w-0 break-words">{suggestion}</span>
              </div>
            ))}
          </div>
        )}
      </AnchoredPopover>
    </div>
  );
}
