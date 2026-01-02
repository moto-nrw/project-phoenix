"use client";

import React, { useState, useRef, useEffect } from "react";
import type { FilterConfig } from "./types";

interface DesktopFiltersProps {
  readonly filters: ReadonlyArray<FilterConfig>;
  readonly className?: string;
}

export function DesktopFilters({
  filters,
  className = "",
}: Readonly<DesktopFiltersProps>) {
  return (
    <div className={`flex gap-2 ${className}`}>
      {filters.map((filter) => (
        <FilterControl key={filter.id} filter={filter} />
      ))}
    </div>
  );
}

// Helper to normalize filter values to array format
function normalizeFilterValues(value: string | string[] | undefined): string[] {
  if (Array.isArray(value)) return value;
  if (value) return [value];
  return [];
}

function FilterControl({ filter }: Readonly<{ filter: FilterConfig }>) {
  if (filter.type === "buttons") {
    const isMulti = !!filter.multiSelect;
    const selectedValues = normalizeFilterValues(filter.value);
    return (
      <div className="flex h-10 rounded-xl bg-white p-1 shadow-sm">
        {filter.options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => {
              if (isMulti) {
                const next = selectedValues.includes(option.value)
                  ? selectedValues.filter((v) => v !== option.value)
                  : [...selectedValues, option.value];
                filter.onChange(next);
              } else {
                filter.onChange(option.value);
              }
            }}
            className={`rounded-lg px-3 text-sm font-medium transition-all ${
              selectedValues.includes(option.value)
                ? "bg-gray-900 text-white"
                : "text-gray-600 hover:text-gray-900"
            } `}
          >
            {option.label}
          </button>
        ))}
      </div>
    );
  }

  if (filter.type === "dropdown") {
    return <DropdownFilter filter={filter} />;
  }

  if (filter.type === "grid") {
    // For desktop, show grid filters as a dropdown with icons
    return <DropdownFilter filter={filter} showIcons />;
  }

  return null;
}

function DropdownFilter({
  filter,
  showIcons = false,
}: Readonly<{
  filter: FilterConfig;
  showIcons?: boolean;
}>) {
  const [isOpen, setIsOpen] = useState(false);
  const [dropdownPosition, setDropdownPosition] = useState<"left" | "right">(
    "left",
  );
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const isMulti = !!filter.multiSelect;
  const selectedValues = normalizeFilterValues(filter.value);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Calculate dropdown position when opening
  useEffect(() => {
    if (isOpen && buttonRef.current) {
      const buttonRect = buttonRef.current.getBoundingClientRect();
      const dropdownWidth = 192; // w-48 = 12rem = 192px
      const windowWidth = window.innerWidth;

      // Check if dropdown would overflow on the right
      if (buttonRect.left + dropdownWidth > windowWidth - 16) {
        // 16px margin
        setDropdownPosition("right");
      } else {
        setDropdownPosition("left");
      }
    }
  }, [isOpen]);

  const selectedOption = isMulti
    ? undefined
    : filter.options.find((opt) => opt.value === filter.value);

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`flex h-10 items-center gap-2 rounded-xl bg-white px-3 py-2 text-sm font-medium whitespace-nowrap shadow-sm transition-all ${filter.value !== filter.options[0]?.value ? "ring-2 ring-blue-500 ring-offset-1" : ""} ${isOpen ? "bg-gray-50" : "hover:bg-gray-50"} `}
      >
        {showIcons && selectedOption?.icon && (
          <svg
            className="h-4 w-4 flex-shrink-0 text-gray-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d={selectedOption.icon}
            />
          </svg>
        )}
        <span
          className={`whitespace-nowrap ${selectedOption && filter.value !== filter.options[0]?.value ? "text-gray-900" : "text-gray-600"}`}
        >
          {isMulti ? filter.label : (selectedOption?.label ?? filter.label)}
        </span>
        <svg
          className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${isOpen ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {isOpen && (
        <div
          className={`absolute top-full z-50 mt-1 w-48 rounded-xl border border-gray-200 bg-white py-1 shadow-lg ${
            dropdownPosition === "right" ? "right-0" : "left-0"
          }`}
        >
          {filter.options.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => {
                if (isMulti) {
                  const next = selectedValues.includes(option.value)
                    ? selectedValues.filter((v) => v !== option.value)
                    : [...selectedValues, option.value];
                  filter.onChange(next);
                } else {
                  filter.onChange(option.value);
                  setIsOpen(false);
                }
              }}
              className={`flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors hover:bg-gray-50 ${selectedValues.includes(option.value) ? "bg-gray-50 font-medium text-gray-900" : "text-gray-700"} `}
            >
              {showIcons && option.icon && (
                <svg
                  className="h-4 w-4 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d={option.icon}
                  />
                </svg>
              )}
              <span className="flex-1">{option.label}</span>
              {isMulti && selectedValues.includes(option.value) && (
                <svg
                  className="h-4 w-4 text-gray-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              )}
              {option.count !== undefined && (
                <span className="ml-1 text-gray-500">({option.count})</span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
