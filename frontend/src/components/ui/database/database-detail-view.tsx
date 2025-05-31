"use client";

import type { ReactNode } from "react";
import type { DatabaseTheme } from "./themes";
import { getThemeClassNames } from "./themes";

export interface DetailSection {
  title: string;
  titleColor?: string; // Override theme color
  items: DetailItem[];
}

export interface DetailItem {
  label: string;
  value: ReactNode;
  isEmpty?: boolean;
  isStatus?: boolean; // For status grid items
  statusColor?: string; // For active/inactive status coloring
}

export interface DatabaseDetailViewProps {
  theme: DatabaseTheme;
  title: string;
  subtitle?: string;
  avatar?: {
    content: string; // e.g., initials
    size?: 'sm' | 'md' | 'lg';
  };
  headerBadges?: ReactNode[];
  sections: DetailSection[];
  onEdit: () => void;
  onDelete: () => void;
  deleteConfirmMessage: string;
  extraInfo?: ReactNode; // Additional info below title/subtitle
}

export function DatabaseDetailView({
  theme,
  title,
  subtitle,
  avatar,
  headerBadges,
  sections,
  onEdit,
  onDelete,
  deleteConfirmMessage,
  extraInfo,
}: DatabaseDetailViewProps) {
  const themeClasses = getThemeClassNames(theme);
  
  const handleDelete = () => {
    if (window.confirm(deleteConfirmMessage)) {
      onDelete();
    }
  };

  // Avatar sizes
  const avatarSizes = {
    sm: 'h-12 w-12 md:h-16 md:w-16 text-lg md:text-xl',
    md: 'h-16 w-16 md:h-20 md:w-20 text-2xl md:text-3xl',
    lg: 'h-20 w-20 md:h-24 md:w-24 text-3xl md:text-4xl',
  };

  return (
    <div className="space-y-6">
      {/* Header with gradient and info - matching StudentDetailView exactly */}
      <div className={`relative -mx-4 md:-mx-6 -mt-4 md:-mt-6 bg-gradient-to-r ${themeClasses.gradient} p-4 md:p-6 text-white`}>
        <div className="flex items-center">
          {avatar && (
            <div className={`mr-3 md:mr-5 flex ${avatarSizes[avatar.size ?? 'md']} items-center justify-center rounded-full bg-white/30 font-bold`}>
              {avatar.content}
            </div>
          )}
          <div>
            <h2 className="text-lg md:text-xl lg:text-2xl font-bold">{title}</h2>
            {subtitle && (
              <p className="text-sm md:text-base opacity-90">{subtitle}</p>
            )}
            {extraInfo}
          </div>
        </div>

        {/* Status badges in top-right corner */}
        {headerBadges && headerBadges.length > 0 && (
          <div className="absolute top-3 md:top-4 lg:top-6 right-3 md:right-4 lg:right-6 flex flex-col space-y-1 md:space-y-2">
            {headerBadges}
          </div>
        )}
      </div>

      {/* Action buttons - matching StudentDetailView exactly */}
      <div className="flex flex-col sm:flex-row gap-2 sm:justify-end">
        <button
          onClick={onEdit}
          className="min-h-[40px] md:min-h-[44px] rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-4 py-2 md:px-6 text-xs md:text-sm font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]"
        >
          Bearbeiten
        </button>
        <button
          onClick={handleDelete}
          className="min-h-[40px] md:min-h-[44px] rounded-lg border border-red-300 bg-white px-3 py-2 md:px-4 text-xs md:text-sm font-medium text-red-600 shadow-sm transition-all duration-200 hover:bg-red-50 active:scale-[0.98]"
        >
          Löschen
        </button>
      </div>

      {/* Details grid - matching StudentDetailView layout */}
      <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
        {sections.map((section, sectionIndex) => {
          // Determine border color
          const borderColor = section.titleColor 
            ? `border-${section.titleColor}-200`
            : themeClasses.border;
          const titleColor = section.titleColor
            ? `text-${section.titleColor}-800`
            : themeClasses.text;

          return (
            <div key={sectionIndex} className="space-y-4">
              <h3 className={`${borderColor} border-b pb-1 md:pb-2 text-sm md:text-base lg:text-lg font-medium ${titleColor}`}>
                {section.title}
              </h3>

              {section.items.map((item, itemIndex) => {
                // Handle status items (like the colored status grid in StudentDetailView)
                if (item.isStatus) {
                  const bgColor = item.statusColor 
                    ? `bg-${item.statusColor}-100` 
                    : 'bg-gray-100';
                  const textColor = item.statusColor
                    ? `text-${item.statusColor}-800`
                    : 'text-gray-500';
                  const dotColor = item.statusColor
                    ? `bg-${item.statusColor}-500`
                    : 'bg-gray-300';

                  return (
                    <div
                      key={itemIndex}
                      className={`rounded-lg p-2 md:p-3 text-sm ${bgColor} ${textColor}`}
                    >
                      <span className="flex items-center">
                        <span
                          className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${dotColor}`}
                        ></span>
                        <span className="truncate">{item.label}</span>
                      </span>
                    </div>
                  );
                }

                // Regular items
                return (
                  <div key={itemIndex}>
                    <div className="text-xs md:text-sm text-gray-500">{item.label}</div>
                    <div className="text-sm md:text-base">
                      {item.isEmpty ? (
                        <span className="text-gray-400">{item.value ?? 'Nicht angegeben'}</span>
                      ) : (
                        item.value
                      )}
                    </div>
                  </div>
                );
              })}

              {/* If this section has status items, wrap them in a grid */}
              {section.items.some(item => item.isStatus) && (
                <div className="grid grid-cols-2 gap-2 md:gap-4">
                  {section.items.filter(item => item.isStatus).map((item, itemIndex) => {
                    const bgColor = item.statusColor 
                      ? `bg-${item.statusColor}-100` 
                      : 'bg-gray-100';
                    const textColor = item.statusColor
                      ? `text-${item.statusColor}-800`
                      : 'text-gray-500';
                    const dotColor = item.statusColor
                      ? `bg-${item.statusColor}-500`
                      : 'bg-gray-300';

                    return (
                      <div
                        key={itemIndex}
                        className={`rounded-lg p-2 md:p-3 text-sm ${bgColor} ${textColor}`}
                      >
                        <span className="flex items-center">
                          <span
                            className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${dotColor}`}
                          ></span>
                          <span className="truncate">{item.label}</span>
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}