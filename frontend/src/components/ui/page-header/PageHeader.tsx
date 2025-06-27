"use client";

import React from "react";
import type { PageHeaderProps } from "./types";

export function PageHeader({ title, badge, className = "" }: PageHeaderProps) {
  return (
    <div className={`mb-6 ${className}`}>
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-[1.625rem] md:text-3xl font-bold text-gray-900">
          {title}
        </h1>
        
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
    </div>
  );
}