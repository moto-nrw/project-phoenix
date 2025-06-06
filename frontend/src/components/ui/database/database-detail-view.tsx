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

export interface DetailItem {
  label: string;
  value: ReactNode;
  isEmpty?: boolean;
  isStatus?: boolean; // For status grid items
  statusColor?: string; // For active/inactive status coloring
}

export interface DatabaseDetailViewProps {
  theme: DatabaseTheme;
  header?: {
    title: string;
    subtitle?: string;
    avatar?: {
      text: string;
      size?: 'sm' | 'md' | 'lg';
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
    sm: 'h-12 w-12 md:h-16 md:w-16 text-lg md:text-xl',
    md: 'h-16 w-16 md:h-20 md:w-20 text-2xl md:text-3xl',
    lg: 'h-20 w-20 md:h-24 md:w-24 text-3xl md:text-4xl',
  };

  return (
    <div className="space-y-6">
      {/* Header with gradient and info - matching StudentDetailView exactly */}
      {header && (
        <div className={`relative -mx-4 md:-mx-6 -mt-4 md:-mt-6 bg-gradient-to-r ${themeClasses.gradient} p-4 md:p-6 text-white`}>
          <div className="flex items-center">
            {header.avatar && (
              <div className={`mr-3 md:mr-5 flex ${avatarSizes[header.avatar.size ?? 'md']} items-center justify-center rounded-full bg-white/30 font-bold`}>
                {header.avatar.text}
              </div>
            )}
            <div>
              <h2 className="text-lg md:text-xl lg:text-2xl font-bold">{header.title}</h2>
              {header.subtitle && (
                <p className="text-sm md:text-base opacity-90">{header.subtitle}</p>
              )}
            </div>
          </div>

          {/* Status badges in top-right corner */}
          {header.badges && header.badges.length > 0 && (
            <div className="absolute top-3 md:top-4 lg:top-6 right-3 md:right-4 lg:right-6 flex flex-col space-y-1 md:space-y-2">
              {header.badges.map((badge, index) => (
                <span key={index} className={`rounded-full ${badge.color} px-3 py-1.5 text-sm font-medium shadow-sm`}>
                  {badge.label}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Action buttons - matching StudentDetailView exactly */}
      {actions && (actions.onEdit ?? actions.onDelete ?? actions.custom) && (
        <div className="flex flex-col sm:flex-row gap-2 sm:justify-end">
          {actions.onEdit && (
            <button
              onClick={actions.onEdit}
              className="min-h-[40px] md:min-h-[44px] rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-4 py-2 md:px-6 text-xs md:text-sm font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]"
            >
              Bearbeiten
            </button>
          )}
          {actions.onDelete && (
            <button
              onClick={handleDelete}
              className="min-h-[40px] md:min-h-[44px] rounded-lg border border-red-300 bg-white px-3 py-2 md:px-4 text-xs md:text-sm font-medium text-red-600 shadow-sm transition-all duration-200 hover:bg-red-50 active:scale-[0.98]"
            >
              Löschen
            </button>
          )}
          {actions.custom?.map((action, index) => (
            <button
              key={index}
              onClick={action.onClick}
              className={`min-h-[40px] md:min-h-[44px] rounded-lg px-4 py-2 md:px-6 text-xs md:text-sm font-medium shadow-sm transition-all duration-200 active:scale-[0.98] ${
                action.color ?? 'bg-gray-100 text-gray-700 hover:bg-gray-200'
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
          const borderColor = section.titleColor?.includes('-')
            ? `border-${section.titleColor.split('-')[0]}-200`
            : `border-${themeClasses.border}`;
          const titleColor = section.titleColor ?? themeClasses.text;

          return (
            <div key={sectionIndex} className="space-y-4">
              <h3 className={`${borderColor} border-b pb-1 md:pb-2 text-sm md:text-base lg:text-lg font-medium ${titleColor}`}>
                {section.title}
              </h3>

              <div className={section.columns === 2 ? 'grid grid-cols-2 gap-2 md:gap-4' : 'space-y-4'}>
                {section.items.map((item, itemIndex) => (
                  <div key={itemIndex}>
                    {typeof item.value === 'string' || typeof item.value === 'number' ? (
                      <>
                        <div className="text-xs md:text-sm text-gray-500">{item.label}</div>
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