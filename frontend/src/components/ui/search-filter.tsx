"use client";

import Link from "next/link";
import { useState } from "react";
import type { ReactNode } from "react";

export interface SearchFilterProps {
  searchPlaceholder?: string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  filters?: ReactNode;
  addButton?: {
    label: string;
    href?: string;
    onClick?: () => void;
  };
  className?: string;
  accent?: 'blue' | 'purple' | 'green' | 'red' | 'indigo' | 'gray' | 'amber' | 'orange' | 'pink' | 'yellow';
}

export function SearchFilter({
  searchPlaceholder = "Suchen...",
  searchValue,
  onSearchChange,
  filters,
  addButton,
  className = "",
  accent = 'blue',
}: SearchFilterProps) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);
  const hasFilters = Boolean(filters);
  const accentMap = {
    blue: { ring: 'focus:ring-blue-500', grad: 'from-blue-500 to-blue-600', hoverGrad: 'hover:from-blue-600 hover:to-blue-700' },
    purple: { ring: 'focus:ring-purple-500', grad: 'from-purple-500 to-purple-600', hoverGrad: 'hover:from-purple-600 hover:to-purple-700' },
    green: { ring: 'focus:ring-green-500', grad: 'from-green-500 to-green-600', hoverGrad: 'hover:from-green-600 hover:to-green-700' },
    red: { ring: 'focus:ring-red-500', grad: 'from-red-500 to-red-600', hoverGrad: 'hover:from-red-600 hover:to-red-700' },
    indigo: { ring: 'focus:ring-indigo-500', grad: 'from-indigo-500 to-indigo-600', hoverGrad: 'hover:from-indigo-600 hover:to-indigo-700' },
    gray: { ring: 'focus:ring-gray-500', grad: 'from-gray-500 to-gray-600', hoverGrad: 'hover:from-gray-600 hover:to-gray-700' },
    amber: { ring: 'focus:ring-amber-500', grad: 'from-amber-500 to-amber-600', hoverGrad: 'hover:from-amber-600 hover:to-amber-700' },
    orange: { ring: 'focus:ring-orange-500', grad: 'from-orange-500 to-orange-600', hoverGrad: 'hover:from-orange-600 hover:to-orange-700' },
    pink: { ring: 'focus:ring-pink-500', grad: 'from-pink-500 to-rose-600', hoverGrad: 'hover:from-pink-600 hover:to-rose-700' },
    yellow: { ring: 'focus:ring-yellow-500', grad: 'from-yellow-500 to-yellow-600', hoverGrad: 'hover:from-yellow-600 hover:to-yellow-700' },
  } as const;
  const acc = accentMap[accent] ?? accentMap.blue;

  return (
    <div className={`mb-6 md:mb-8 ${className}`}>
      {/* Mobile Search Bar - Always Visible */}
      <div className="mb-4 md:hidden">
        <div className="relative">
          <input
            type="text"
            placeholder={searchPlaceholder}
            value={searchValue}
            onChange={(e) => onSearchChange(e.target.value)}
            className={`w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 text-base transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 ${acc.ring} focus:outline-none`}
          />
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
            <svg
              xmlns="http://www.w3.org/2000/svg"
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
          </div>
        </div>
      </div>

      {/* Mobile Filter Toggle - Only show if filters exist */}
      {hasFilters && (
        <div className="mb-4 md:hidden">
          <button
            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
            className={`flex w-full items-center justify-between rounded-lg bg-white px-4 py-3 shadow-sm ring-1 ring-gray-200 hover:ring-gray-300 focus:ring-2 ${acc.ring} focus:outline-none transition-all duration-200`}
          >
            <span className="text-sm font-medium text-gray-700">
              Filter & Optionen
            </span>
            <svg
              className={`h-5 w-5 text-gray-400 transition-transform duration-200 ${
                isMobileFiltersOpen ? "rotate-180" : ""
              }`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7 7"
              />
            </svg>
          </button>
        </div>
      )}

      {/* Search and Filter Panel - Responsive */}
      <div
        className={`${
          hasFilters && !isMobileFiltersOpen ? "hidden md:block" : ""
        }`}
      >
        <div className="rounded-lg bg-white p-4 md:p-6 shadow-md border border-gray-100">
          <div className="flex flex-col gap-4">
            {/* Desktop Search and Add Button Row */}
            <div className="hidden md:flex items-center justify-between gap-4">
              <div className="relative max-w-md flex-1">
                <input
                  type="text"
                  placeholder={searchPlaceholder}
                  value={searchValue}
                  onChange={(e) => onSearchChange(e.target.value)}
                  className={`w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 ${acc.ring} focus:outline-none`}
                />
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
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
                </div>
              </div>

              {addButton && (
                addButton.href ? (
                  <Link href={addButton.href}>
                    <button className={`group flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r ${acc.grad} px-6 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 ${acc.hoverGrad}`}>
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M12 4v16m8-8H4"
                        />
                      </svg>
                      <span>{addButton.label}</span>
                    </button>
                  </Link>
                ) : (
                  <button 
                    onClick={addButton.onClick}
                    className={`group flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r ${acc.grad} px-6 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 ${acc.hoverGrad}`}
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 4v16m8-8H4"
                      />
                    </svg>
                    <span>{addButton.label}</span>
                  </button>
                )
              )}
            </div>

            {/* Filter Section - If filters are provided */}
            {filters && (
              <div className="flex flex-col md:flex-row md:items-center gap-3 md:gap-4">
                <span className="text-sm font-medium text-gray-700">Filter:</span>
                <div className="flex-1">{filters}</div>
              </div>
            )}

            {/* Mobile Add Button */}
            {addButton && (
              <div className="md:hidden">
                {addButton.href ? (
                  <Link href={addButton.href}>
                    <button className={`group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r ${acc.grad} px-4 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 ${acc.hoverGrad} active:scale-[0.98]`}>
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M12 4v16m8-8H4"
                        />
                      </svg>
                      <span>{addButton.label}</span>
                    </button>
                  </Link>
                ) : (
                  <button 
                    onClick={addButton.onClick}
                    className={`group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r ${acc.grad} px-4 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 ${acc.hoverGrad} active:scale-[0.98]`}
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 4v16m8-8H4"
                      />
                    </svg>
                    <span>{addButton.label}</span>
                  </button>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
