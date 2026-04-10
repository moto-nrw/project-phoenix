"use client";

import { useState, useEffect, useCallback, type ReactNode } from "react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";

interface Tab {
  id: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
}

function MobileBackHeader({
  onBack,
  tabs: tabList,
  activeTab,
}: {
  readonly onBack: () => void;
  readonly tabs: Tab[];
  readonly activeTab: string | null;
}) {
  return (
    <div className="mb-4 flex items-center gap-3 pb-4">
      <button
        onClick={onBack}
        className="-ml-3 rounded-lg p-3 transition-all hover:bg-gray-100 active:bg-gray-200"
        aria-label="Zurück"
      >
        <svg
          className="h-5 w-5 text-gray-700"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M15 19l-7-7 7-7"
          />
        </svg>
      </button>
      <h2 className="text-lg font-semibold text-gray-900">
        {tabList.find((t) => t.id === activeTab)?.label}
      </h2>
    </div>
  );
}

interface MobileTabCardProps {
  readonly tab: Tab;
  readonly onSelect: () => void;
}

function MobileTabCard({ tab, onSelect }: MobileTabCardProps) {
  return (
    <div className="mx-4">
      <button
        onClick={onSelect}
        className="flex w-full items-center justify-between rounded-2xl bg-white px-4 py-4 shadow-sm transition-colors hover:bg-gray-50 active:bg-gray-100"
      >
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-gray-100">
            <svg
              className="h-5 w-5 text-gray-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d={tab.icon}
              />
            </svg>
          </div>
          <p className="text-base text-gray-900">{tab.label}</p>
          {tab.adminOnly && (
            <span className="rounded bg-gray-200 px-1.5 py-0.5 text-xs">
              Admin
            </span>
          )}
        </div>
        <svg
          className="h-5 w-5 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5l7 7-7 7"
          />
        </svg>
      </button>
    </div>
  );
}

interface SettingsLayoutProps {
  readonly tabs: Tab[];
  readonly renderTab: (tabId: string) => ReactNode;
}

export function SettingsLayout({ tabs, renderTab }: SettingsLayoutProps) {
  const [activeTab, setActiveTab] = useState<string | null>(
    tabs[0]?.id ?? null,
  );
  const [isMobile, setIsMobile] = useState(false);

  const handleBackToList = useCallback(() => {
    setActiveTab(null);
  }, []);

  useEffect(() => {
    const handleResize = () => {
      const wasDesktop = !isMobile;
      const isNowMobile = window.innerWidth < 768;
      setIsMobile(isNowMobile);

      if (wasDesktop && isNowMobile) {
        setActiveTab(null);
      } else if (!wasDesktop && !isNowMobile && activeTab === null) {
        setActiveTab(tabs[0]?.id ?? null);
      }
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile, activeTab, tabs]);

  return (
    <div className="-mt-1.5 w-full">
      {isMobile && activeTab === null && (
        <PageHeaderWithSearch title="Einstellungen" />
      )}

      {!isMobile && (
        <div className="mb-6 ml-6">
          <Tabs
            value={activeTab ?? tabs[0]?.id ?? ""}
            onValueChange={setActiveTab}
          >
            <TabsList variant="line">
              {tabs.map((tab) => (
                <TabsTrigger key={tab.id} value={tab.id}>
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d={tab.icon}
                    />
                  </svg>
                  {tab.label}
                  {tab.adminOnly && (
                    <span className="ml-1 rounded bg-gray-200 px-1.5 py-0.5 text-xs">
                      Admin
                    </span>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      )}

      {isMobile && (
        <>
          {activeTab === null ? (
            <div className="flex flex-col space-y-2 pb-6">
              {tabs.map((tab) => (
                <MobileTabCard
                  key={tab.id}
                  tab={tab}
                  onSelect={() => setActiveTab(tab.id)}
                />
              ))}
            </div>
          ) : (
            <div className="flex h-[calc(100vh-120px)] flex-col">
              <MobileBackHeader
                onBack={handleBackToList}
                tabs={tabs}
                activeTab={activeTab}
              />
              <div className="-mx-4 flex-1 overflow-y-auto px-4">
                {renderTab(activeTab)}
              </div>
            </div>
          )}
        </>
      )}

      {!isMobile && activeTab && (
        <div className="min-h-[60vh]">{renderTab(activeTab)}</div>
      )}
    </div>
  );
}
