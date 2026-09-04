"use client";

import React from "react";
import { useLatest } from "~/lib/hooks/use-latest";
import type { SearchBarProps } from "./types";

interface DebouncedSearchValue {
  readonly shownValue: string;
  readonly handleChange: (next: string) => void;
  readonly commitNow: (next: string) => void;
}

const DebouncedSearchValueContext =
  React.createContext<DebouncedSearchValue | null>(null);

/**
 * Local draft of the typed text, reported upwards only after `debounceMs` of
 * silence (#2975). Without a debounce the owning page re-renders per keystroke;
 * with one, only this input does. `value` keeps winning when the owner reports
 * a new term; `resetKey` makes an external reset explicit when that term is
 * already the current controlled value.
 */
function useDebouncedSearchValue(
  value: string,
  onChange: (next: string) => void,
  debounceMs: number | undefined,
  resetKey: string | number | undefined,
): DebouncedSearchValue {
  const [draft, setDraft] = React.useState(value);
  const timeoutRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const onChangeRef = useLatest(onChange);

  React.useEffect(() => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    setDraft(value);
  }, [resetKey, value]);

  React.useEffect(
    () => () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    },
    [],
  );

  const handleChange = React.useCallback(
    (next: string) => {
      if (!debounceMs) {
        onChangeRef.current(next);
        return;
      }
      setDraft(next);
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      timeoutRef.current = setTimeout(() => {
        onChangeRef.current(next);
      }, debounceMs);
    },
    [debounceMs, onChangeRef],
  );

  // The clear button is an explicit action, not typing: it reports at once so
  // the list is back immediately instead of after the debounce window.
  const commitNow = React.useCallback(
    (next: string) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      setDraft(next);
      onChangeRef.current(next);
    },
    [onChangeRef],
  );

  return { shownValue: debounceMs ? draft : value, handleChange, commitNow };
}

interface SearchBarDraftProviderProps {
  readonly value: string;
  readonly onChange: (next: string) => void;
  readonly debounceMs: number | undefined;
  readonly resetKey: string | number | undefined;
  readonly children: React.ReactNode;
}

/** Shares one typed draft between explicit responsive copies of one search field. */
export function SearchBarDraftProvider({
  value,
  onChange,
  debounceMs,
  resetKey,
  children,
}: Readonly<SearchBarDraftProviderProps>) {
  const draftState = useDebouncedSearchValue(
    value,
    onChange,
    debounceMs,
    resetKey,
  );

  return (
    <DebouncedSearchValueContext.Provider value={draftState}>
      {children}
    </DebouncedSearchValueContext.Provider>
  );
}

function SearchBarContent({
  placeholder = "Name suchen...",
  onClear,
  className = "",
  size = "md",
  inputProps,
  draftState,
}: Readonly<SearchBarProps & { readonly draftState: DebouncedSearchValue }>) {
  const { shownValue, handleChange, commitNow } = draftState;
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

function SearchBarWithOwnDraft(props: Readonly<SearchBarProps>) {
  const draftState = useDebouncedSearchValue(
    props.value,
    props.onChange,
    props.debounceMs,
    props.resetKey,
  );

  return <SearchBarContent {...props} draftState={draftState} />;
}

export function SearchBar(props: Readonly<SearchBarProps>) {
  return <SearchBarWithOwnDraft {...props} />;
}

/**
 * Renders a responsive copy that uses the draft supplied by
 * `SearchBarDraftProvider`. Kept separate from `SearchBar` so a search field
 * inside a filter or action stays independent of the page header.
 */
export function SharedSearchBar(props: Readonly<SearchBarProps>) {
  const sharedDraftState = React.useContext(DebouncedSearchValueContext);

  if (!sharedDraftState) {
    throw new Error(
      "SharedSearchBar must be rendered inside SearchBarDraftProvider",
    );
  }

  return <SearchBarContent {...props} draftState={sharedDraftState} />;
}
