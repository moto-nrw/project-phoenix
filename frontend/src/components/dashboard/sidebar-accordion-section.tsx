"use client";

import type { ReactNode } from "react";

interface SidebarAccordionSectionProps {
  readonly icon: string;
  readonly label: string;
  readonly activeColor?: string;
  readonly isExpanded: boolean;
  readonly onToggle: () => void;
  readonly isActive: boolean;
  readonly isIconActive?: boolean;
  readonly isLoading?: boolean;
  readonly emptyText?: string;
  readonly children?: ReactNode;
  readonly hasChildren: boolean;
}

export function SidebarAccordionSection({
  icon,
  label,
  activeColor,
  isExpanded,
  onToggle,
  isActive,
  isIconActive,
  isLoading = false,
  emptyText = "Keine Einträge",
  children,
  hasChildren,
}: SidebarAccordionSectionProps) {
  const headerBase =
    "flex items-center px-3 py-2.5 text-sm lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm rounded-lg transition-colors";
  const headerActive = "bg-gray-100 text-gray-900 font-semibold";
  const headerInactive =
    "text-gray-600 hover:bg-gray-50 hover:text-gray-900 font-medium";

  const iconBase =
    "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5 transition-colors";
  const iconColorClass =
    (isIconActive ?? isActive) && activeColor ? activeColor : "";

  return (
    <div>
      {/* Header row — single box with icon, label, and chevron */}
      <button
        type="button"
        onClick={onToggle}
        className={`${headerBase} ${isActive ? headerActive : headerInactive} w-full cursor-pointer appearance-none border-0 bg-transparent p-0 text-left`}
        aria-expanded={isExpanded}
      >
        <svg
          className={`${iconBase} ${iconColorClass}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d={icon}
          />
        </svg>
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <svg
          className={`ml-auto h-4 w-4 shrink-0 text-gray-400 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {/* Expandable body — CSS Grid transition */}
      <div
        className={`grid transition-[grid-template-rows] duration-200 ease-in-out ${
          isExpanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
        }`}
      >
        <div className="overflow-hidden">
          <div className="py-0.5">
            {isLoading && (
              /* Skeleton shimmer */
              <div className="space-y-1 pr-3 pl-10">
                <div className="h-7 w-3/4 animate-pulse rounded bg-gray-100" />
                <div className="h-7 w-2/3 animate-pulse rounded bg-gray-100" />
                <div className="h-7 w-1/2 animate-pulse rounded bg-gray-100" />
              </div>
            )}
            {!isLoading && hasChildren && children}
            {!isLoading && !hasChildren && (
              /* Empty state */
              <div className="py-2 pr-3 pl-10 text-xs text-gray-400">
                {emptyText}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
