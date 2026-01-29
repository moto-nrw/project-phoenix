"use client";

import { useState, useEffect, useCallback } from "react";
import type {
  SettingTab,
  TabSettingsResponse,
  Scope,
} from "~/lib/settings-helpers";
import {
  clientFetchSettingsTabs,
  clientFetchTabSettings,
} from "~/lib/settings-api";
import { SettingsTabNav } from "./settings-tab-nav";
import { SettingsCategory } from "./settings-category";
import { Loading } from "~/components/ui/loading";

export function SystemSettingsTab() {
  const [tabs, setTabs] = useState<SettingTab[]>([]);
  const [activeTab, setActiveTab] = useState<string | null>(null);
  const [tabSettings, setTabSettings] = useState<TabSettingsResponse | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [tabLoading, setTabLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Settings are edited at system scope for admin
  const scope: Scope = "system";

  // Load available tabs
  useEffect(() => {
    let isMounted = true;

    async function loadTabs() {
      try {
        const fetchedTabs = await clientFetchSettingsTabs();
        if (isMounted) {
          setTabs(fetchedTabs);
          // Auto-select first tab if available
          if (fetchedTabs.length > 0 && fetchedTabs[0]) {
            setActiveTab(fetchedTabs[0].key);
          }
        }
      } catch (err) {
        if (isMounted) {
          setError("Fehler beim Laden der Einstellungen");
          console.error("Error loading settings tabs:", err);
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    }

    void loadTabs();

    return () => {
      isMounted = false;
    };
  }, []);

  // Load tab settings when active tab changes
  useEffect(() => {
    if (!activeTab) return;

    // Copy to local variable for type narrowing inside async function
    const currentTab = activeTab;
    let isMounted = true;

    async function loadTabSettings() {
      setTabLoading(true);
      try {
        const settings = await clientFetchTabSettings(currentTab);
        if (isMounted) {
          setTabSettings(settings);
        }
      } catch (err) {
        if (isMounted) {
          console.error("Error loading tab settings:", err);
        }
      } finally {
        if (isMounted) {
          setTabLoading(false);
        }
      }
    }

    void loadTabSettings();

    return () => {
      isMounted = false;
    };
  }, [activeTab]);

  const handleTabChange = useCallback((tab: string) => {
    setActiveTab(tab);
    setTabSettings(null);
  }, []);

  const handleSettingChanged = useCallback(() => {
    // Optionally refresh the tab settings after a change
    if (activeTab !== null) {
      void clientFetchTabSettings(activeTab).then(setTabSettings);
    }
  }, [activeTab]);

  if (loading) {
    return (
      <div className="flex justify-center py-8">
        <Loading fullPage={false} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4">
        <p className="text-sm text-red-600">{error}</p>
      </div>
    );
  }

  if (tabs.length === 0) {
    return (
      <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
        <p className="text-sm text-gray-600">
          Keine Systemeinstellungen verfügbar.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Tab navigation for settings categories */}
      <SettingsTabNav
        tabs={tabs}
        activeTab={activeTab ?? ""}
        onTabChange={handleTabChange}
      />

      {/* Tab content */}
      {tabLoading ? (
        <div className="flex justify-center py-8">
          <Loading fullPage={false} />
        </div>
      ) : tabSettings ? (
        <div className="space-y-6">
          {tabSettings.categories.map((category) => (
            <SettingsCategory
              key={category.key}
              category={category}
              scope={scope}
              onSettingChanged={handleSettingChanged}
            />
          ))}
          {tabSettings.categories.length === 0 && (
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
              <p className="text-sm text-gray-600">
                Keine Einstellungen in diesem Tab verfügbar.
              </p>
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
