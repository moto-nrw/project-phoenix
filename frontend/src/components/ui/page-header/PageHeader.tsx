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
    <div className={`mb-6 md:hidden ${className}`}>
      <div className="flex items-end justify-between gap-4">
        {/* Title with underline */}
        <div className="relative ml-6">
          <h1 className="text-2xl font-bold text-gray-900 pb-3">
            {title}
          </h1>
          {/* Underline indicator - matches tab style */}
          <div
            className="absolute bottom-0 left-0 h-0.5 bg-gray-900 rounded-full"
            style={{ width: '80%' }}
          />
        </div>

        {/* Badge and Status */}
        {(statusIndicator ?? badge) && (
          <div className="flex items-center gap-3 pb-3 mr-4">
            {/* Status Indicator Dot */}
            {statusIndicator && (
              <div
                className={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${getStatusColor(statusIndicator.color)} ${statusIndicator.color === 'green' ? 'animate-pulse' : ''}`}
                title={statusIndicator.tooltip}
              />
            )}

            {/* Badge */}
            {badge && (
              <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-50 rounded-full border border-gray-100">
                {badge.icon && (
                  <span className="text-gray-500">{badge.icon}</span>
                )}
                <span className="text-sm font-semibold text-gray-900">
                  {badge.count}
                </span>
                {badge.label && (
                  <span className="text-xs text-gray-500">
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
