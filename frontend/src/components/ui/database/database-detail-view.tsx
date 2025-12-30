"use client";

import type { ReactNode } from "react";
import type { DatabaseTheme } from "./themes";
import { getThemeClassNames } from "./themes";

export interface DetailSection {
  title: string;
  titleColor?: string; // Override theme color
  items: Array<{
    label: string;
    value: ReactNode;
  }>;
  columns?: 1 | 2;
}

export interface DatabaseDetailViewProps {
  theme: DatabaseTheme;
  header?: {
    title: string;
    subtitle?: string;
    avatar?: {
      text: string;
      size?: "sm" | "md" | "lg";
    };
    badges?: Array<{
      label: string;
      color: string;
    }>;
  };
  sections: DetailSection[];
  actions?: {
    onEdit?: () => void;
    onDelete?: () => void;
    custom?: Array<{
      label: string;
      onClick: () => void;
      color?: string;
    }>;
  };
}

export function DatabaseDetailView({
  theme,
  header,
  sections,
  actions,
}: DatabaseDetailViewProps) {
  const themeClasses = getThemeClassNames(theme);

  const handleDelete = () => {
    if (actions?.onDelete) {
      actions.onDelete();
    }
  };

  // Avatar sizes
  const avatarSizes = {
    sm: "h-12 w-12 md:h-16 md:w-16 text-lg md:text-xl",
    md: "h-16 w-16 md:h-20 md:w-20 text-2xl md:text-3xl",
    lg: "h-20 w-20 md:h-24 md:w-24 text-3xl md:text-4xl",
  };

  return (
    <div className="space-y-6">
      {/* Header with gradient and info - matching StudentDetailView exactly */}
      {header && (
        <div
          className={`relative -mx-4 -mt-4 bg-gradient-to-r md:-mx-6 md:-mt-6 ${themeClasses.gradient} p-4 text-white md:p-6`}
        >
          <div className="flex items-center">
            {header.avatar && (
              <div
                className={`mr-3 flex md:mr-5 ${avatarSizes[header.avatar.size ?? "md"]} items-center justify-center rounded-full bg-white/30 font-bold`}
              >
                {header.avatar.text}
              </div>
            )}
            <div>
              <h2 className="text-lg font-bold md:text-xl lg:text-2xl">
                {header.title}
              </h2>
              {header.subtitle && (
                <p className="text-sm opacity-90 md:text-base">
                  {header.subtitle}
                </p>
              )}
            </div>
          </div>

          {/* Status badges in top-right corner */}
          {header.badges && header.badges.length > 0 && (
            <div className="absolute top-3 right-3 flex flex-col space-y-1 md:top-4 md:right-4 md:space-y-2 lg:top-6 lg:right-6">
              {header.badges.map((badge, index) => (
                <span
                  key={index}
                  className={`rounded-full ${badge.color} px-3 py-1.5 text-sm font-medium shadow-sm`}
                >
                  {badge.label}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Action buttons - matching StudentDetailView exactly */}
      {actions && (actions.onEdit ?? actions.onDelete ?? actions.custom) && (
        <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
          {actions.onEdit && (
            <button
              onClick={actions.onEdit}
              className="min-h-[40px] rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-4 py-2 text-xs font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98] md:min-h-[44px] md:px-6 md:text-sm"
            >
              Bearbeiten
            </button>
          )}
          {actions.onDelete && (
            <button
              onClick={handleDelete}
              className="min-h-[40px] rounded-lg border border-red-300 bg-white px-3 py-2 text-xs font-medium text-red-600 shadow-sm transition-all duration-200 hover:bg-red-50 active:scale-[0.98] md:min-h-[44px] md:px-4 md:text-sm"
            >
              Löschen
            </button>
          )}
          {actions.custom?.map((action, index) => (
            <button
              key={index}
              onClick={action.onClick}
              className={`min-h-[40px] rounded-lg px-4 py-2 text-xs font-medium shadow-sm transition-all duration-200 active:scale-[0.98] md:min-h-[44px] md:px-6 md:text-sm ${
                action.color ?? "bg-gray-100 text-gray-700 hover:bg-gray-200"
              }`}
            >
              {action.label}
            </button>
          ))}
        </div>
      )}

      {/* Details grid - matching StudentDetailView layout */}
      <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
        {sections.map((section, sectionIndex) => {
          // Determine border color
          const borderColor = section.titleColor?.includes("-")
            ? `border-${section.titleColor.split("-")[0]}-200`
            : `border-${themeClasses.border}`;
          const titleColor = section.titleColor ?? themeClasses.text;

          return (
            <div key={sectionIndex} className="space-y-4">
              <h3
                className={`${borderColor} border-b pb-1 text-sm font-medium md:pb-2 md:text-base lg:text-lg ${titleColor}`}
              >
                {section.title}
              </h3>

              <div
                className={
                  section.columns === 2
                    ? "grid grid-cols-2 gap-2 md:gap-4"
                    : "space-y-4"
                }
              >
                {section.items.map((item, itemIndex) => (
                  <div key={itemIndex}>
                    {typeof item.value === "string" ||
                    typeof item.value === "number" ? (
                      <>
                        <div className="text-xs text-gray-500 md:text-sm">
                          {item.label}
                        </div>
                        <div className="text-sm md:text-base">{item.value}</div>
                      </>
                    ) : (
                      item.value
                    )}
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
