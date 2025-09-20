"use client";

import { type ReactNode } from "react";
import Link from "next/link";

export interface DatabaseListItemProps {
  id: string | number;
  href?: string;
  onClick?: () => void;
  title: string;
  subtitle?: ReactNode;
  badges?: ReactNode[];
  leftIcon?: ReactNode;
  rightInfo?: ReactNode;
  indicator?: {
    type: "color" | "dot" | "icon";
    value: string | ReactNode;
  };
  className?: string;
  minHeight?: "sm" | "md" | "lg";
}

const minHeightStyles = {
  sm: "min-h-[60px] md:min-h-[72px]",
  md: "min-h-[60px] md:min-h-[80px]",
  lg: "min-h-[72px] md:min-h-[96px]",
};

export function DatabaseListItem({
  id: _id,
  href,
  onClick,
  title,
  subtitle,
  badges,
  leftIcon,
  rightInfo,
  indicator,
  className = "",
  minHeight = "sm",
}: DatabaseListItemProps) {
  const handleClick = () => {
    if (onClick) {
      onClick();
    }
  };

  const content = (
    <>
      <div className="flex items-center space-x-3 md:space-x-4 min-w-0 flex-1">
        {/* Left Icon/Avatar */}
        {leftIcon && (
          <div className="flex-shrink-0">
            {leftIcon}
          </div>
        )}

        {/* Main Content Area */}
        <div className="flex flex-col min-w-0 flex-1 transition-transform duration-200 group-hover:translate-x-1">
          {/* Title and Indicator Row */}
          <div className="flex items-center gap-2">
            <span className="font-semibold text-gray-900 transition-colors duration-200 group-hover:text-blue-600 truncate">
              {title}
            </span>
            {indicator?.type === "dot" && typeof indicator.value === "string" && (
              <span className={`inline-block w-2 h-2 rounded-full ${indicator.value}`} />
            )}
          </div>

          {/* Subtitle if provided */}
          {subtitle && (
            <div className="mt-0.5 text-sm text-gray-600">
              {subtitle}
            </div>
          )}

          {/* Badges Row */}
          {badges && badges.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-1.5 md:gap-2">
              {badges}
            </div>
          )}
        </div>

        {/* Right Info Section */}
        {rightInfo && (
          <div className="flex-shrink-0 text-sm text-gray-500">
            {rightInfo}
          </div>
        )}
      </div>

      {/* Indicator - Color Bar on Left */}
      {indicator?.type === "color" && typeof indicator.value === "string" && (
        <div 
          className={`absolute left-0 top-0 bottom-0 w-1 rounded-l-lg ${indicator.value}`}
        />
      )}

      {/* Indicator - Custom Icon */}
      {indicator?.type === "icon" && (
        <div className="flex-shrink-0">
          {indicator.value}
        </div>
      )}

      {/* Arrow Icon - Always Present */}
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500 flex-shrink-0"
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
    </>
  );

  const baseClasses = `group flex cursor-pointer items-center justify-between rounded-lg border border-gray-200 bg-white p-3 md:p-4 transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.99] relative ${minHeightStyles[minHeight]} ${className}`;

  // If href is provided, wrap in Link
  if (href) {
    return (
      <Link href={href} onClick={handleClick}>
        <div className={baseClasses}>
          {content}
        </div>
      </Link>
    );
  }

  // Otherwise, render as div with onClick
  return (
    <div onClick={handleClick} className={baseClasses}>
      {content}
    </div>
  );
}