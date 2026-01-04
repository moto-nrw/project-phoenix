"use client";

import React, { useRef, useEffect, useState, useMemo } from "react";
import type { NavigationTabsProps } from "./types";

export function NavigationTabs({
  items,
  activeTab,
  onTabChange,
  className = "",
}: Readonly<NavigationTabsProps>) {
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });

  // Memoize active index to avoid redundant findIndex calls on re-renders
  const activeIndex = useMemo(
    () => items.findIndex((item) => item.id === activeTab),
    [items, activeTab],
  );

  // Update sliding indicator position when active tab changes
  useEffect(() => {
    const activeTabElement = tabRefs.current[activeIndex];

    if (activeTabElement) {
      const { offsetLeft, offsetWidth } = activeTabElement;
      setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
    }
  }, [activeIndex]);

  return (
    <div className={`ml-3 md:ml-6 ${className}`}>
      {/* Modern underline tabs with sliding indicator */}
      <div className="relative flex gap-4 md:gap-8">
        {items.map((tab, index) => {
          const isActive = activeTab === tab.id;

          return (
            <button
              key={tab.id}
              ref={(el) => {
                tabRefs.current[index] = el;
              }}
              type="button"
              onClick={() => onTabChange(tab.id)}
              className={`relative px-0 pb-3 text-sm font-medium transition-all md:text-base ${
                isActive
                  ? "font-semibold text-gray-900"
                  : "text-gray-500 hover:text-gray-700"
              } `}
            >
              <span className="whitespace-nowrap">{tab.label}</span>
            </button>
          );
        })}

        {/* Sliding indicator bar */}
        <div
          className="absolute bottom-0 h-0.5 rounded-full bg-gray-900 transition-all duration-300 ease-out"
          style={{
            left: `${indicatorStyle.left}px`,
            width: `${indicatorStyle.width}px`,
          }}
        />
      </div>
    </div>
  );
}
