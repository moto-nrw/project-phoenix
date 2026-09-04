"use client";

import React from "react";
import type { SearchBarProps } from "./types";

/**
 * Local draft of the typed text, reported upwards only after `debounceMs` of
 * silence (#2975). Without a debounce the owning page re-renders per keystroke;
 * with one, only this input does. `value` keeps winning: whenever the owner
 * reports a different term than the one this field last sent (a cleared chip,
 * a reset filter), the field follows.
 */
function useDebouncedSearchValue(
  value: string,
  onChange: (next: string) => void,
  debounceMs: number | undefined,
) {
  const [draft, setDraft] = React.useState(value);
  // The last value this field pushed upwards. Comparing against it separates
  // "the owner changed the term" from "the owner echoed our own change back".
  const lastSentRef = React.useRef(value);
  const timeoutRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  React.useEffect(() => {
    if (value === lastSentRef.current) return;
    lastSentRef.current = value;
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    setDraft(value);
  }, [value]);

  React.useEffect(
    () => () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    },
    [],
  );

  const handleChange = React.useCallback(
    (next: string) => {
      if (!debounceMs) {
        lastSentRef.current = next;
        onChange(next);
        return;
      }
      setDraft(next);
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      timeoutRef.current = setTimeout(() => {
        lastSentRef.current = next;
        onChange(next);
      }, debounceMs);
    },
    [debounceMs, onChange],
  );

  // The clear button is an explicit action, not typing: it reports at once so
  // the list is back immediately instead of after the debounce window.
  const commitNow = React.useCallback(
    (next: string) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      setDraft(next);
      lastSentRef.current = next;
      onChange(next);
    },
    [onChange],
  );

  return { shownValue: debounceMs ? draft : value, handleChange, commitNow };
}

export function SearchBar({
  value,
  onChange,
  placeholder = "Name suchen...",
  onClear,
  className = "",
  size = "md",
  inputProps,
  debounceMs,
}: Readonly<SearchBarProps>) {
  const { shownValue, handleChange, commitNow } = useDebouncedSearchValue(
    value,
    onChange,
    debounceMs,
  );
  const sizeClasses = {
    sm: "py-2 pl-9 pr-3 text-sm",
    md: "py-2.5 pl-9 pr-3 text-sm md:pl-10 md:pr-10",
    lg: "py-3 pl-10 pr-10 text-base",
  };

  const iconSizeClasses = {
    sm: "h-4 w-4",
    md: "h-4 w-4",
    lg: "h-5 w-5",
  };

  return (
    <div className={`relative ${className}`}>
      <svg
        className={`absolute top-1/2 left-3 -translate-y-1/2 transform ${iconSizeClasses[size]} text-gray-400`}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>

      <input
        {...inputProps}
        type="text"
        placeholder={placeholder}
        value={shownValue}
        onChange={(e) => handleChange(e.target.value)}
        className={`w-full rounded-2xl border border-gray-200 bg-white text-gray-900 placeholder-gray-400 transition-all duration-200 focus:border-gray-300 focus:ring-0 focus:outline-none ${sizeClasses[size]} ${shownValue ? "pr-10" : ""} `}
      />

      {shownValue && (
        <button
          type="button"
          aria-label="Suche löschen"
          onClick={() => {
            commitNow("");
            onClear?.();
          }}
          className="absolute top-1/2 right-2 -translate-y-1/2 transform rounded-full p-1 transition-colors hover:bg-gray-100 md:right-3"
        >
          <svg
            className={`${iconSizeClasses[size]} text-gray-400`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      )}
    </div>
  );
}
