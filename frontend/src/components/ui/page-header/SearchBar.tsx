"use client";

import React from "react";
import type { SearchBarProps } from "./types";

export function SearchBar({
  value,
  onChange,
  placeholder = "Name suchen...",
  onClear,
  className = "",
  size = "md"
}: SearchBarProps) {
  const sizeClasses = {
    sm: "py-2 pl-9 pr-3 text-sm",
    md: "py-2.5 pl-9 pr-3 text-sm md:pl-10 md:pr-10",
    lg: "py-3 pl-10 pr-10 text-base"
  };

  const iconSizeClasses = {
    sm: "h-4 w-4",
    md: "h-4 w-4",
    lg: "h-5 w-5"
  };

  return (
    <div className={`relative ${className}`}>
      <svg 
        className={`absolute left-3 top-1/2 transform -translate-y-1/2 ${iconSizeClasses[size]} text-gray-400`} 
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
        type="text"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`
          w-full bg-white border border-gray-200 rounded-2xl text-gray-900 
          placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500 
          focus:border-blue-500 transition-all duration-200
          ${sizeClasses[size]}
          ${value ? 'pr-10' : ''}
        `}
      />
      
      {value && (
        <button
          type="button"
          onClick={() => {
            onChange("");
            onClear?.();
          }}
          className="absolute right-2 md:right-3 top-1/2 transform -translate-y-1/2 p-1 hover:bg-gray-100 rounded-full transition-colors"
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