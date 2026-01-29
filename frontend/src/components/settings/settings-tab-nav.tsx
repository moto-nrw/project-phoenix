"use client";

import type { SettingTab } from "~/lib/settings-helpers";

interface SettingsTabNavProps {
  tabs: SettingTab[];
  activeTab: string;
  onTabChange: (tab: string) => void;
}

export function SettingsTabNav({
  tabs,
  activeTab,
  onTabChange,
}: SettingsTabNavProps) {
  return (
    <div className="border-b border-gray-200">
      <nav className="-mb-px flex space-x-8" aria-label="Tabs">
        {tabs.map((tab) => {
          const isActive = tab.key === activeTab;
          return (
            <button
              key={tab.key}
              onClick={() => onTabChange(tab.key)}
              className={`whitespace-nowrap border-b-2 px-1 py-4 text-sm font-medium ${
                isActive
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700"
              }`}
              aria-current={isActive ? "page" : undefined}
            >
              {tab.icon && <span className="mr-2">{tab.icon}</span>}
              {tab.name}
            </button>
          );
        })}
      </nav>
    </div>
  );
}
