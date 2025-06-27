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
    <div className={`flex gap-1 mb-6 ${className}`}>
      {items.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onTabChange(tab.id)}
          className={`
            px-4 py-2 rounded-lg font-medium transition-all duration-200
            ${activeTab === tab.id
              ? 'bg-gray-900 text-white'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200 hover:text-gray-900'
            }
          `}
        >
          <span className="flex items-center gap-2">
            {tab.label}
            {tab.count !== undefined && (
              <span className={`
                text-xs px-1.5 py-0.5 rounded-full
                ${activeTab === tab.id ? 'bg-gray-700' : 'bg-gray-200'}
              `}>
                {tab.count}
              </span>
            )}
          </span>
        </button>
      ))}
    </div>
  );
}