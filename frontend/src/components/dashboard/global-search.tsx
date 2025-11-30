// components/dashboard/global-search.tsx
"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useDebounce } from "~/lib/use-debounce";

// Search result interfaces
interface SearchResult {
  id: string;
  type: "student" | "room" | "activity" | "group" | "teacher";
  title: string;
  subtitle?: string;
  description?: string;
  status?: string;
  href: string;
  icon: React.ReactNode;
  badge?: {
    text: string;
    color: "blue" | "green" | "amber" | "purple" | "red" | "gray";
  };
}

interface GlobalSearchProps {
  className?: string;
  placeholder?: string;
  onResultSelect?: () => void;
}

// Search result type icons
const getSearchIcon = (type: string) => {
  switch (type) {
    case "student":
      return (
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
          />
        </svg>
      );
    case "room":
      return (
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
          />
        </svg>
      );
    case "activity":
      return (
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
          />
        </svg>
      );
    case "group":
      return (
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
          />
        </svg>
      );
    case "teacher":
      return (
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.746 0 3.332.477 4.5 1.253v13C19.832 18.477 18.246 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
          />
        </svg>
      );
    default:
      return (
        <svg
          className="h-4 w-4"
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
      );
  }
};

// Badge color styles
const getBadgeStyles = (color: string) => {
  switch (color) {
    case "blue":
      return "bg-blue-100 text-blue-800";
    case "green":
      return "bg-green-100 text-green-800";
    case "amber":
      return "bg-amber-100 text-amber-800";
    case "purple":
      return "bg-purple-100 text-purple-800";
    case "red":
      return "bg-red-100 text-red-800";
    case "gray":
      return "bg-gray-100 text-gray-800";
    default:
      return "bg-gray-100 text-gray-800";
  }
};

// Mock search function - will be replaced with real API calls
const mockSearch = async (query: string): Promise<SearchResult[]> => {
  // Simulate API delay
  await new Promise((resolve) => setTimeout(resolve, 200));

  if (!query.trim()) return [];

  const mockResults: SearchResult[] = [
    // Students
    {
      id: "1",
      type: "student",
      title: "Max Mustermann",
      subtitle: "Klasse 4a",
      description: "Im Haus • Letzte Aktivität: vor 5 min",
      href: "/students/1",
      icon: getSearchIcon("student"),
      badge: { text: "Anwesend", color: "green" },
    },
    {
      id: "2",
      type: "student",
      title: "Emma Schmidt",
      subtitle: "Klasse 3b",
      description: "Schulhof • Letzte Aktivität: vor 12 min",
      href: "/students/2",
      icon: getSearchIcon("student"),
      badge: { text: "Draußen", color: "blue" },
    },
    // Rooms
    {
      id: "3",
      type: "room",
      title: "Turnhalle A",
      subtitle: "Gebäude Haupthaus",
      description: "Erdgeschoss • Kapazität: 50 • Belegt",
      href: "/rooms/3",
      icon: getSearchIcon("room"),
      badge: { text: "Belegt", color: "red" },
    },
    {
      id: "4",
      type: "room",
      title: "Klassenraum 2.15",
      subtitle: "Gebäude Haupthaus",
      description: "2. OG • Kapazität: 25 • Verfügbar",
      href: "/rooms/4",
      icon: getSearchIcon("room"),
      badge: { text: "Verfügbar", color: "green" },
    },
    // Activities
    {
      id: "5",
      type: "activity",
      title: "Fußball AG",
      subtitle: "Sport",
      description: "12/15 Teilnehmer • Leitung: Herr Wagner",
      href: "/activities/5",
      icon: getSearchIcon("activity"),
      badge: { text: "Aktiv", color: "green" },
    },
    // Groups
    {
      id: "6",
      type: "group",
      title: "Sonnenschein",
      subtitle: "OGS Gruppe",
      description: "Klasse 1-2 • 24 Kinder • Raum 1.12",
      href: "/ogs-groups",
      icon: getSearchIcon("group"),
      badge: { text: "Aktiv", color: "green" },
    },
    // Teachers
    {
      id: "7",
      type: "teacher",
      title: "Frau Müller",
      subtitle: "Grundschullehrerin",
      description: "Deutsch, Sachkunde • Klasse 3a",
      href: "/database/teachers/7",
      icon: getSearchIcon("teacher"),
      badge: { text: "Verfügbar", color: "blue" },
    },
  ];

  // Filter results based on query
  return mockResults.filter(
    (result) =>
      result.title.toLowerCase().includes(query.toLowerCase()) ||
      (result.subtitle?.toLowerCase().includes(query.toLowerCase()) ?? false) ||
      (result.description?.toLowerCase().includes(query.toLowerCase()) ??
        false),
  );
};

export function GlobalSearch({
  className = "",
  placeholder = "Schüler, Räume, Aktivitäten suchen...",
  onResultSelect,
}: GlobalSearchProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  const inputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);
  const router = useRouter();

  // Debounce search query
  const debouncedQuery = useDebounce(query, 300);

  // Perform search
  const performSearch = useCallback(async (searchQuery: string) => {
    if (!searchQuery.trim()) {
      setResults([]);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    try {
      const searchResults = await mockSearch(searchQuery);
      setResults(searchResults);
    } catch (error) {
      console.error("Search error:", error);
      setResults([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Effect for debounced search
  useEffect(() => {
    void performSearch(debouncedQuery);
  }, [debouncedQuery, performSearch]);

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setQuery(e.target.value);
    setSelectedIndex(-1);
    setIsOpen(true);
  };

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex((prev) => (prev < results.length - 1 ? prev + 1 : 0));
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : results.length - 1));
        break;
      case "Enter":
        e.preventDefault();
        if (selectedIndex >= 0 && selectedIndex < results.length) {
          const selectedResult = results[selectedIndex];
          if (selectedResult) {
            handleResultSelect(selectedResult);
          }
        }
        break;
      case "Escape":
        setIsOpen(false);
        setSelectedIndex(-1);
        inputRef.current?.blur();
        break;
    }
  };

  // Handle result selection
  const handleResultSelect = (result: SearchResult) => {
    setQuery("");
    setResults([]);
    setIsOpen(false);
    setSelectedIndex(-1);
    onResultSelect?.();
    router.push(result.href);
  };

  // Handle focus/blur
  const handleFocus = () => {
    if (query.trim() && results.length > 0) {
      setIsOpen(true);
    }
  };

  const handleBlur = () => {
    // Delay closing to allow for result clicks
    void setTimeout(() => {
      if (!resultsRef.current?.contains(document.activeElement)) {
        setIsOpen(false);
        setSelectedIndex(-1);
      }
    }, 150);
  };

  // Keyboard shortcut (Cmd+K or Ctrl+K)
  useEffect(() => {
    const handleKeyboard = (e: KeyboardEvent) => {
      if ((e.metaKey ?? e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };

    document.addEventListener("keydown", handleKeyboard);
    return () => document.removeEventListener("keydown", handleKeyboard);
  }, []);

  return (
    <div className={`relative ${className}`}>
      {/* Search Input */}
      <div className="relative">
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          <svg
            className="h-4 w-4 text-gray-400"
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
        </div>

        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          onFocus={handleFocus}
          onBlur={handleBlur}
          placeholder={placeholder}
          className="block w-full rounded-lg border border-gray-200 bg-gray-50/50 py-2 pr-12 pl-9 text-sm placeholder-gray-500 transition-colors duration-200 hover:bg-white/80 focus:border-transparent focus:ring-2 focus:ring-[#5080d8]/50 focus:outline-none"
          autoComplete="off"
          spellCheck="false"
        />

        {/* Loading indicator */}
        <div className="absolute inset-y-0 right-0 flex items-center pr-3">
          {isLoading && (
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-[#5080d8]"></div>
          )}
        </div>
      </div>

      {/* Results Dropdown */}
      {isOpen && (query.trim() || results.length > 0) && (
        <div
          ref={resultsRef}
          className="absolute top-full right-0 left-0 z-50 mt-2 max-h-96 overflow-y-auto rounded-xl border border-gray-200 bg-white shadow-xl"
        >
          {results.length > 0 ? (
            <>
              {/* Results Header */}
              <div className="border-b border-gray-100 bg-gray-50/50 px-3 py-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-gray-500">
                    {results.length} Ergebnis{results.length !== 1 ? "se" : ""}{" "}
                    gefunden
                  </span>
                  <span className="text-xs text-gray-400">
                    ↑↓ navigieren • ↵ auswählen • esc schließen
                  </span>
                </div>
              </div>

              {/* Results List */}
              <div className="py-2">
                {results.map((result, index) => (
                  <button
                    key={result.id}
                    onClick={() => handleResultSelect(result)}
                    className={`w-full px-3 py-3 text-left transition-colors duration-150 hover:bg-gray-50 focus:bg-gray-50 focus:outline-none active:bg-gray-100 ${
                      index === selectedIndex
                        ? "border-r-2 border-[#5080d8] bg-[#5080d8]/5"
                        : ""
                    }`}
                  >
                    <div className="flex items-start space-x-3">
                      {/* Icon */}
                      <div
                        className={`mt-0.5 rounded-lg p-1.5 ${
                          index === selectedIndex
                            ? "bg-[#5080d8]/10 text-[#5080d8]"
                            : "bg-gray-100 text-gray-600"
                        }`}
                      >
                        {result.icon}
                      </div>

                      {/* Content */}
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center space-x-2">
                            <h4 className="truncate text-sm font-medium text-gray-900">
                              {result.title}
                            </h4>
                            {result.badge && (
                              <span
                                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getBadgeStyles(result.badge.color)}`}
                              >
                                {result.badge.text}
                              </span>
                            )}
                          </div>
                        </div>

                        {result.subtitle && (
                          <p className="mt-0.5 truncate text-xs text-gray-500">
                            {result.subtitle}
                          </p>
                        )}

                        {result.description && (
                          <p className="mt-1 line-clamp-2 text-xs text-gray-400">
                            {result.description}
                          </p>
                        )}
                      </div>

                      {/* Arrow indicator */}
                      <div className="mt-1">
                        <svg
                          className="h-4 w-4 text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M9 5l7 7-7 7"
                          />
                        </svg>
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            </>
          ) : query.trim() && !isLoading ? (
            /* No Results */
            <div className="px-3 py-8 text-center">
              <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
                <svg
                  className="h-6 w-6 text-gray-400"
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
              </div>
              <h3 className="mb-1 text-sm font-medium text-gray-900">
                Keine Ergebnisse gefunden
              </h3>
              <p className="text-xs text-gray-500">
                Versuche andere Suchbegriffe.
              </p>
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
