"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Image from "next/image";

interface Organization {
  id: string;
  name: string;
  slug: string;
}

function getBaseDomain(): string {
  if (typeof window === "undefined") {
    return process.env.NEXT_PUBLIC_BASE_DOMAIN ?? "localhost:3000";
  }
  return process.env.NEXT_PUBLIC_BASE_DOMAIN ?? window.location.host;
}

function getProtocol(): string {
  if (typeof window === "undefined") {
    return "http:";
  }
  return window.location.protocol;
}

function buildSubdomainUrl(slug: string): string {
  const baseDomain = getBaseDomain();
  const protocol = getProtocol();

  // Handle localhost development
  if (baseDomain.startsWith("localhost")) {
    return `${protocol}//${slug}.${baseDomain}/login`;
  }

  // Production: slug.domain.com/login
  return `${protocol}//${slug}.${baseDomain}/login`;
}

export function OrgSelection() {
  const [search, setSearch] = useState("");
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Debounced search
  const searchOrganizations = useCallback(async (query: string) => {
    setIsLoading(true);
    try {
      const params = new URLSearchParams();
      if (query.trim()) {
        params.set("search", query);
      }
      params.set("limit", "10");

      const response = await fetch(`/api/public/organizations?${params}`);
      if (response.ok) {
        const data = (await response.json()) as Organization[];
        setOrganizations(data);
      }
    } catch (error) {
      console.error("Failed to search organizations:", error);
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Initial load
  useEffect(() => {
    void searchOrganizations("");
  }, [searchOrganizations]);

  // Debounced search on input change
  useEffect(() => {
    const timeout = setTimeout(() => {
      void searchOrganizations(search);
    }, 300);
    return () => clearTimeout(timeout);
  }, [search, searchOrganizations]);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setShowDropdown(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showDropdown || organizations.length === 0) return;

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex((prev) =>
          prev < organizations.length - 1 ? prev + 1 : prev,
        );
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
        break;
      case "Enter":
        e.preventDefault();
        if (selectedIndex >= 0 && organizations[selectedIndex]) {
          handleSelectOrg(organizations[selectedIndex]);
        }
        break;
      case "Escape":
        setShowDropdown(false);
        break;
    }
  };

  const handleSelectOrg = (org: Organization) => {
    // Redirect to subdomain login page
    const url = buildSubdomainUrl(org.slug);
    window.location.href = url;
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto w-full max-w-xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        {/* Welcome Text */}
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-3xl font-bold text-transparent md:text-4xl">
          Willkommen bei moto!
        </h1>
        <p className="mb-8 text-lg text-gray-700">
          Wählen Sie Ihre Einrichtung
        </p>

        {/* Search Input */}
        <div className="relative" ref={dropdownRef}>
          <div className="relative">
            <input
              ref={inputRef}
              type="text"
              placeholder="Einrichtung suchen..."
              value={search}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                setSearch(e.target.value);
                setShowDropdown(true);
                setSelectedIndex(-1);
              }}
              onFocus={() => setShowDropdown(true)}
              onKeyDown={handleKeyDown}
              className="w-full rounded-lg border border-gray-300 px-4 py-3 pr-10 text-gray-900 placeholder-gray-500 focus:border-blue-500 focus:ring-2 focus:ring-blue-500 focus:outline-none"
            />
            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
              {isLoading ? (
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-gray-300 border-t-gray-600" />
              ) : (
                <svg
                  className="h-5 w-5 text-gray-400"
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
              )}
            </div>
          </div>

          {/* Dropdown Results */}
          {showDropdown && organizations.length > 0 && (
            <div className="absolute z-10 mt-2 w-full rounded-lg border border-gray-200 bg-white shadow-lg">
              <ul className="max-h-64 overflow-auto py-2">
                {organizations.map((org, index) => (
                  <li key={org.id}>
                    <button
                      type="button"
                      onClick={() => handleSelectOrg(org)}
                      onMouseEnter={() => setSelectedIndex(index)}
                      className={`flex w-full items-center justify-between px-4 py-3 text-left transition-colors ${
                        index === selectedIndex
                          ? "bg-blue-50"
                          : "hover:bg-gray-50"
                      }`}
                    >
                      <span className="text-gray-600 italic">{org.name}</span>
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
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* No Results */}
          {showDropdown &&
            !isLoading &&
            organizations.length === 0 &&
            search && (
              <div className="absolute z-10 mt-2 w-full rounded-lg border border-gray-200 bg-white p-4 text-center shadow-lg">
                <p className="text-gray-500">Keine Einrichtung gefunden</p>
              </div>
            )}
        </div>

        {/* Sign Up Link */}
        <div className="mt-6 text-sm text-gray-600">
          <p>
            Neue Einrichtung registrieren?{" "}
            <a
              href="/signup"
              className="font-medium text-gray-900 underline hover:text-gray-700"
            >
              Jetzt registrieren
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}
