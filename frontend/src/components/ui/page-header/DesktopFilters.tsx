"use client";

import React, { useState, useRef, useEffect } from "react";
import type { FilterConfig } from "./types";

interface DesktopFiltersProps {
  filters: FilterConfig[];
  className?: string;
}

export function DesktopFilters({ filters, className = "" }: DesktopFiltersProps) {
  return (
    <div className={`flex gap-2 ${className}`}>
      {filters.map((filter) => (
        <FilterControl key={filter.id} filter={filter} />
      ))}
    </div>
  );
}

function FilterControl({ filter }: { filter: FilterConfig }) {
  if (filter.type === 'buttons') {
    return (
      <div className="flex bg-white rounded-xl p-1 shadow-sm h-10">
        {filter.options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => filter.onChange(option.value)}
            className={`
              px-3 rounded-lg text-sm font-medium transition-all
              ${filter.value === option.value 
                ? 'bg-gray-900 text-white' 
                : 'text-gray-600 hover:text-gray-900'
              }
            `}
          >
            {option.label}
          </button>
        ))}
      </div>
    );
  }

  if (filter.type === 'dropdown') {
    return <DropdownFilter filter={filter} />;
  }

  if (filter.type === 'grid') {
    // For desktop, show grid filters as a dropdown with icons
    return <DropdownFilter filter={filter} showIcons />;
  }

  return null;
}

function DropdownFilter({ filter, showIcons = false }: { filter: FilterConfig; showIcons?: boolean }) {
  const [isOpen, setIsOpen] = useState(false);
  const [dropdownPosition, setDropdownPosition] = useState<'left' | 'right'>('left');
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Calculate dropdown position when opening
  useEffect(() => {
    if (isOpen && buttonRef.current) {
      const buttonRect = buttonRef.current.getBoundingClientRect();
      const dropdownWidth = 192; // w-48 = 12rem = 192px
      const windowWidth = window.innerWidth;
      
      // Check if dropdown would overflow on the right
      if (buttonRect.left + dropdownWidth > windowWidth - 16) { // 16px margin
        setDropdownPosition('right');
      } else {
        setDropdownPosition('left');
      }
    }
  }, [isOpen]);

  const selectedOption = filter.options.find(opt => opt.value === filter.value);

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`
          flex items-center gap-2 px-3 py-2 bg-white rounded-xl shadow-sm text-sm font-medium transition-all h-10 whitespace-nowrap
          ${filter.value !== filter.options[0]?.value ? 'ring-2 ring-blue-500 ring-offset-1' : ''}
          ${isOpen ? 'bg-gray-50' : 'hover:bg-gray-50'}
        `}
      >
        {showIcons && selectedOption?.icon && (
          <svg className="h-4 w-4 text-gray-600 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={selectedOption.icon} />
          </svg>
        )}
        <span className={`whitespace-nowrap ${filter.value !== filter.options[0]?.value ? 'text-gray-900' : 'text-gray-600'}`}>
          {selectedOption?.label ?? filter.label}
        </span>
        <svg
          className={`h-4 w-4 text-gray-400 flex-shrink-0 transition-transform ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className={`absolute top-full mt-1 w-48 bg-white rounded-xl shadow-lg border border-gray-200 py-1 z-50 ${
          dropdownPosition === 'right' ? 'right-0' : 'left-0'
        }`}>
          {filter.options.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => {
                filter.onChange(option.value);
                setIsOpen(false);
              }}
              className={`
                w-full text-left px-4 py-2 text-sm hover:bg-gray-50 transition-colors flex items-center gap-2
                ${filter.value === option.value ? 'bg-gray-50 font-medium text-gray-900' : 'text-gray-700'}
              `}
            >
              {showIcons && option.icon && (
                <svg className="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={option.icon} />
                </svg>
              )}
              <span className="flex-1">{option.label}</span>
              {option.count !== undefined && (
                <span className="text-gray-500 ml-1">({option.count})</span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}