"use client";

import React from "react";
import type { NavigationTabsProps } from "./types";

export function NavigationTabs({
  items,
  activeTab,
  onTabChange,
  className = ""
}: NavigationTabsProps) {
  return (
    <div className={`mb-4 ${className}`}>
      {/* Desktop: Settings-style tab selector */}
      <div className="inline-block bg-white/50 backdrop-blur-sm rounded-xl p-1 border border-gray-100">
        <div className="flex gap-1">
          {items.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => onTabChange(tab.id)}
              className={`
                flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-all whitespace-nowrap
                ${activeTab === tab.id
                  ? 'bg-gray-900 text-white shadow-md'
                  : 'text-gray-600 hover:bg-gray-200/80'
                }
              `}
            >
              <span>{tab.label}</span>
              {tab.count !== undefined && (
                <span className={`
                  text-xs px-1.5 py-0.5 rounded-full
                  ${activeTab === tab.id ? 'bg-gray-700' : 'bg-gray-200'}
                `}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}