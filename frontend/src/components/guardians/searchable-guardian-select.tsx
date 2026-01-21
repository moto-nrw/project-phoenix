"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { Search, Loader2, Users, X } from "lucide-react";
import type { GuardianSearchResult } from "@/lib/guardian-helpers";
import { searchGuardiansWithStudents } from "@/lib/guardian-api";

interface SearchableGuardianSelectProps {
  readonly onSelect: (guardian: GuardianSearchResult) => void;
  readonly excludeStudentId?: string;
  readonly disabled?: boolean;
}

export default function SearchableGuardianSelect({
  onSelect,
  excludeStudentId,
  disabled = false,
}: SearchableGuardianSelectProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<GuardianSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showDropdown, setShowDropdown] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Debounced search function
  const performSearch = useCallback(
    async (searchQuery: string) => {
      if (searchQuery.length < 2) {
        setResults([]);
        setError(null);
        return;
      }

      setIsSearching(true);
      setError(null);

      try {
        const searchResults = await searchGuardiansWithStudents(
          searchQuery,
          excludeStudentId,
        );
        setResults(searchResults);
        setShowDropdown(true);
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Fehler bei der Suche",
        );
        setResults([]);
      } finally {
        setIsSearching(false);
      }
    },
    [excludeStudentId],
  );

  // Handle query change with debounce
  useEffect(() => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    if (query.length < 2) {
      setResults([]);
      setShowDropdown(false);
      return;
    }

    debounceTimerRef.current = setTimeout(() => {
      void performSearch(query);
    }, 300);

    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, [query, performSearch]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(event.target as Node)
      ) {
        setShowDropdown(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  // Handle guardian selection
  const handleSelect = (guardian: GuardianSearchResult) => {
    onSelect(guardian);
    setQuery("");
    setResults([]);
    setShowDropdown(false);
  };

  // Clear search
  const handleClear = () => {
    setQuery("");
    setResults([]);
    setShowDropdown(false);
    inputRef.current?.focus();
  };

  // Format student list as a readable string
  const formatStudentList = (students: GuardianSearchResult["students"]) => {
    if (students.length === 0) return null;
    return students
      .map((s) => `${s.firstName} (${s.schoolClass})`)
      .join(", ");
  };

  return (
    <div className="relative">
      {/* Search Input */}
      <div className="relative">
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          {isSearching ? (
            <Loader2 className="h-4 w-4 animate-spin text-gray-400" />
          ) : (
            <Search className="h-4 w-4 text-gray-400" />
          )}
        </div>
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => {
            if (results.length > 0) {
              setShowDropdown(true);
            }
          }}
          placeholder="Name oder E-Mail eingeben (min. 2 Zeichen)"
          className="block w-full rounded-lg border border-gray-200 bg-white py-2.5 pr-10 pl-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] disabled:cursor-not-allowed disabled:bg-gray-100"
          disabled={disabled}
          autoComplete="off"
        />
        {query && (
          <button
            type="button"
            onClick={handleClear}
            className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600"
            aria-label="Suche leeren"
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Error Message */}
      {error && (
        <p className="mt-1.5 text-xs text-red-600">{error}</p>
      )}

      {/* Results Dropdown */}
      {showDropdown && results.length > 0 && (
        <div
          ref={dropdownRef}
          className="absolute z-50 mt-1 max-h-60 w-full overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg"
        >
          {results.map((guardian) => (
            <button
              key={guardian.id}
              type="button"
              onClick={() => handleSelect(guardian)}
              className="flex w-full flex-col items-start gap-1 border-b border-gray-100 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-gray-50"
            >
              {/* Guardian Name and Contact */}
              <div className="flex w-full items-center justify-between gap-2">
                <span className="font-medium text-gray-900">
                  {guardian.firstName} {guardian.lastName}
                </span>
                {guardian.email && (
                  <span className="truncate text-xs text-gray-500">
                    {guardian.email}
                  </span>
                )}
              </div>

              {/* Linked Students */}
              {guardian.students.length > 0 && (
                <div className="flex items-center gap-1.5 text-xs text-gray-600">
                  <Users className="h-3 w-3 flex-shrink-0" />
                  <span>
                    Geschwister: {formatStudentList(guardian.students)}
                  </span>
                </div>
              )}

              {/* No students indicator */}
              {guardian.students.length === 0 && (
                <span className="text-xs text-gray-400">
                  Noch keinem Schüler zugeordnet
                </span>
              )}
            </button>
          ))}
        </div>
      )}

      {/* No Results Message */}
      {showDropdown && query.length >= 2 && results.length === 0 && !isSearching && !error && (
        <div
          ref={dropdownRef}
          className="absolute z-50 mt-1 w-full rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-lg"
        >
          <p className="text-sm text-gray-500">
            Keine Erziehungsberechtigten gefunden
          </p>
        </div>
      )}

      {/* Help Text */}
      <p className="mt-1.5 text-xs text-gray-500">
        Suchen Sie nach dem Namen oder der E-Mail-Adresse eines bestehenden Erziehungsberechtigten
      </p>
    </div>
  );
}
