"use client";

import React, { useRef, useEffect, useState } from "react";
import type { NavigationTabsProps } from "./types";

export function NavigationTabs({
  items,
  activeTab,
  onTabChange,
  className = ""
}: NavigationTabsProps) {
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });

  // Update sliding indicator position when active tab changes
  useEffect(() => {
    const activeIndex = items.findIndex(item => item.id === activeTab);
    const activeTabElement = tabRefs.current[activeIndex];

    if (activeTabElement) {
      const { offsetLeft, offsetWidth } = activeTabElement;
      setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
    }
  }, [activeTab, items]);

  return (
    <div className={`ml-6 ${className}`}>
      {/* Modern underline tabs with sliding indicator */}
      <div className="relative flex gap-8">
        {items.map((tab, index) => {
          const isActive = activeTab === tab.id;

          return (
            <button
              key={tab.id}
              ref={el => { tabRefs.current[index] = el; }}
              type="button"
              onClick={() => onTabChange(tab.id)}
              className={`
                relative pb-3 text-base font-medium transition-all px-0
                ${isActive
                  ? 'text-gray-900 font-semibold'
                  : 'text-gray-500 hover:text-gray-700'
                }
              `}
            >
              <span>{tab.label}</span>
            </button>
          );
        })}

        {/* Sliding indicator bar */}
        <div
          className="absolute bottom-0 h-0.5 bg-gray-900 transition-all duration-300 ease-out rounded-full"
          style={{
            left: `${indicatorStyle.left}px`,
            width: `${indicatorStyle.width}px`
          }}
        />
      </div>
    </div>
  );
}