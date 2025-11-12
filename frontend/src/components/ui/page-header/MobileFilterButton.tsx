"use client";

import React from "react";

interface MobileFilterButtonProps {
  isOpen: boolean;
  onClick: () => void;
  hasActiveFilters: boolean;
  className?: string;
}

export function MobileFilterButton({
  isOpen,
  onClick,
  hasActiveFilters,
  className = "",
}: MobileFilterButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-2xl p-2.5 transition-all duration-200 ${
        isOpen
          ? "bg-blue-500 text-white"
          : "border border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
      } ${hasActiveFilters && !isOpen ? "ring-2 ring-blue-500 ring-offset-1" : ""} ${className} `}
    >
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
          d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
        />
      </svg>
    </button>
  );
}
