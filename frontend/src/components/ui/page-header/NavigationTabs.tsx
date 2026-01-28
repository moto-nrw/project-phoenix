"use client";

import React, {
  useRef,
  useEffect,
  useState,
  useMemo,
  useCallback,
} from "react";
import type { NavigationTabsProps } from "./types";

export function NavigationTabs({
  items,
  activeTab,
  onTabChange,
  className = "",
}: Readonly<NavigationTabsProps>) {
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const showMobileDropdown = items.length >= 3;

  // Memoize active index to avoid redundant findIndex calls on re-renders
  const activeIndex = useMemo(
    () => items.findIndex((item) => item.id === activeTab),
    [items, activeTab],
  );

  const activeLabel = items[activeIndex]?.label ?? "";

  // Check scroll overflow state
  const updateScrollState = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 0);
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
  }, []);

  // Update sliding indicator position when active tab changes
  const updateIndicator = useCallback(() => {
    const activeTabElement = tabRefs.current[activeIndex];

    if (activeTabElement && activeTabElement.offsetWidth > 0) {
      const { offsetLeft, offsetWidth } = activeTabElement;
      setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
    }
  }, [activeIndex]);

  useEffect(() => {
    updateIndicator();

    // Scroll active tab into view on mobile
    const activeTabElement = tabRefs.current[activeIndex];
    if (activeTabElement) {
      const container = scrollRef.current;
      if (container) {
        const tabLeft = activeTabElement.offsetLeft;
        const tabRight = tabLeft + activeTabElement.offsetWidth;
        const containerLeft = container.scrollLeft;
        const containerRight = containerLeft + container.clientWidth;

        if (tabLeft < containerLeft) {
          container.scrollTo({ left: tabLeft - 16, behavior: "smooth" });
        } else if (tabRight > containerRight) {
          container.scrollTo({
            left: tabRight - container.clientWidth + 16,
            behavior: "smooth",
          });
        }
      }
    }
  }, [activeIndex, updateIndicator]);

  // Monitor scroll state and indicator on mount, resize, and tab changes
  useEffect(() => {
    updateScrollState();
    updateIndicator();
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener("scroll", updateScrollState, { passive: true });
    const observer = new ResizeObserver(() => {
      updateScrollState();
      updateIndicator();
    });
    observer.observe(el);
    return () => {
      el.removeEventListener("scroll", updateScrollState);
      observer.disconnect();
    };
  }, [updateScrollState, updateIndicator, items]);

  return (
    <div className={`ml-3 md:ml-6 ${className}`}>
      {/* Mobile dropdown for 3+ items */}
      {showMobileDropdown && (
        <MobileTabDropdown
          items={items}
          activeTab={activeTab}
          activeLabel={activeLabel}
          onTabChange={onTabChange}
        />
      )}

      {/* Tabs — hidden on mobile when dropdown is shown */}
      <div
        className={`relative ${showMobileDropdown ? "hidden md:block" : ""}`}
      >
        {/* Left fade indicator */}
        {canScrollLeft && (
          <div className="pointer-events-none absolute top-0 bottom-0 left-0 z-10 w-6 bg-gradient-to-r from-white to-transparent md:hidden" />
        )}

        {/* Scrollable tabs container */}
        <div
          ref={scrollRef}
          className="scrollbar-hidden relative flex gap-4 overflow-x-auto md:gap-8"
        >
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

        {/* Right fade indicator */}
        {canScrollRight && (
          <div className="pointer-events-none absolute top-0 right-0 bottom-0 z-10 w-6 bg-gradient-to-l from-white to-transparent md:hidden" />
        )}
      </div>
    </div>
  );
}

function MobileTabDropdown({
  items,
  activeTab,
  activeLabel,
  onTabChange,
}: Readonly<{
  items: NavigationTabsProps["items"];
  activeTab: string;
  activeLabel: string;
  onTabChange: (id: string) => void;
}>) {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="relative md:hidden" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`flex items-center gap-2 rounded-xl bg-white px-4 py-2.5 text-base font-semibold shadow-sm transition-all ${
          isOpen ? "bg-gray-50" : "hover:bg-gray-50"
        }`}
      >
        <span className="text-gray-900">{activeLabel}</span>
        <svg
          className={`h-5 w-5 flex-shrink-0 text-gray-400 transition-transform ${isOpen ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 z-50 mt-1 min-w-48 rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
          {items.map((item) => {
            const isActive = item.id === activeTab;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => {
                  onTabChange(item.id);
                  setIsOpen(false);
                }}
                className={`w-full px-4 py-2.5 text-left text-base transition-colors ${
                  isActive
                    ? "bg-gray-50 font-semibold text-gray-900"
                    : "text-gray-600 hover:bg-gray-50"
                }`}
              >
                {item.label}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
