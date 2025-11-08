"use client";

import { type ReactNode } from "react";
import Link from "next/link";
import { getAccent } from "./database/accents";

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
  accent?:
    | "blue"
    | "purple"
    | "green"
    | "red"
    | "indigo"
    | "gray"
    | "amber"
    | "orange"
    | "pink"
    | "yellow";
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
  accent = "blue",
}: DatabaseListItemProps) {
  const accentCls = getAccent(accent ?? "blue").listHover;

  const handleClick = () => {
    if (onClick) {
      onClick();
    }
  };

  const content = (
    <>
      <div className="flex min-w-0 flex-1 items-center space-x-3 md:space-x-4">
        {/* Left Icon/Avatar */}
        {leftIcon && <div className="flex-shrink-0">{leftIcon}</div>}

        {/* Main Content Area */}
        <div className="flex min-w-0 flex-1 flex-col transition-transform duration-200 group-hover:translate-x-1">
          {/* Title and Indicator Row */}
          <div className="flex items-center gap-2">
            <span
              className={`truncate font-semibold text-gray-900 transition-colors duration-200 ${accentCls.title}`}
            >
              {title}
            </span>
            {indicator?.type === "dot" &&
              typeof indicator.value === "string" && (
                <span
                  className={`inline-block h-2 w-2 rounded-full ${indicator.value}`}
                />
              )}
          </div>

          {/* Subtitle if provided */}
          {subtitle && (
            <div className="mt-0.5 text-sm text-gray-600">{subtitle}</div>
          )}

          {/* Badges Row */}
          {badges && badges.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-1.5 md:gap-2">{badges}</div>
          )}
        </div>

        {/* Right Info Section */}
        {rightInfo && (
          <div className="flex-shrink-0 text-sm text-gray-500">{rightInfo}</div>
        )}
      </div>

      {/* Indicator - Color Bar on Left */}
      {indicator?.type === "color" && typeof indicator.value === "string" && (
        <div
          className={`absolute top-0 bottom-0 left-0 w-1 rounded-l-lg ${indicator.value}`}
        />
      )}

      {/* Indicator - Custom Icon */}
      {indicator?.type === "icon" && (
        <div className="flex-shrink-0">{indicator.value}</div>
      )}

      {/* Arrow Icon - Always Present */}
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className={`h-5 w-5 flex-shrink-0 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform ${accentCls.arrow}`}
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

  const baseClasses = `group flex cursor-pointer items-center justify-between rounded-lg border border-gray-200 bg-white p-3 md:p-4 transition-all duration-200 ${accentCls.border} hover:shadow-md active:scale-[0.99] relative ${minHeightStyles[minHeight]} ${className}`;

  // If href is provided, wrap in Link
  if (href) {
    return (
      <Link href={href} onClick={handleClick}>
        <div className={baseClasses}>{content}</div>
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
