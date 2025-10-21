"use client";

import React from "react";
import type { PageHeaderProps } from "./types";

export function PageHeader({ title, badge, statusIndicator, className = "" }: PageHeaderProps) {
  const getStatusColor = (color: 'green' | 'yellow' | 'red' | 'gray') => {
    switch (color) {
      case 'green': return 'bg-green-500';
      case 'yellow': return 'bg-yellow-500';
      case 'red': return 'bg-red-500';
      case 'gray': return 'bg-gray-400';
    }
  };

  // Don't render anything if no title (conditional title pattern)
  if (!title) {
    return null;
  }

  return (
    <div className={`mb-4 ${className}`}>
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl md:text-3xl font-bold text-gray-900 tracking-tight">
          {title}
        </h1>

        {(statusIndicator ?? badge) && (
          <div className="flex items-center gap-2">
            {/* Status Indicator Dot */}
            {statusIndicator && (
              <div
                className={`h-2 w-2 rounded-full ${getStatusColor(statusIndicator.color)} ${statusIndicator.color === 'green' ? 'animate-pulse' : ''}`}
                title={statusIndicator.tooltip}
              />
            )}

            {/* Badge */}
            {badge && (
              <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-100 rounded-full">
                {badge.icon && (
                  <span className="text-gray-600">{badge.icon}</span>
                )}
                <span className="text-sm font-medium text-gray-700">
                  {badge.count}
                </span>
                {badge.label && (
                  <span className="text-sm text-gray-600">
                    {badge.label}
                  </span>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}