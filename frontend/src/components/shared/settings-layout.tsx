"use client";

import { useState, useEffect, useCallback, type ReactNode } from "react";
import { useSearchParams } from "next/navigation";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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
    <div className="mb-4 flex items-center gap-2 pb-4">
      <button
        type="button"
        onClick={onBack}
        className="-ml-2 rounded-xl p-2 transition-all active:bg-gray-100"
        aria-label="Zurück"
      >
        <svg
          className="h-5 w-5 text-gray-900"
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

/** iOS-style grouped list for mobile settings navigation */
function MobileTabList({
  tabs,
  onSelect,
}: {
  readonly tabs: Tab[];
  readonly onSelect: (id: string) => void;
}) {
  return (
    <div className="mx-4 overflow-hidden rounded-xl bg-white shadow-sm">
      {tabs.map((tab, idx) => (
        <button
          type="button"
          key={tab.id}
          onClick={() => onSelect(tab.id)}
          className={`flex w-full items-center gap-3 px-4 py-3.5 text-left transition-colors active:bg-gray-50 ${
            idx < tabs.length - 1 ? "border-b border-gray-100" : ""
          }`}
        >
          <svg
            className="h-[18px] w-[18px] shrink-0 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d={tab.icon}
            />
          </svg>
          <span className="flex-1 text-[15px] text-gray-900">{tab.label}</span>
          {tab.adminOnly && (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500">
              Admin
            </span>
          )}
          <svg
            className="h-4 w-4 shrink-0 text-gray-300"
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
      ))}
    </div>
  );
}

interface SettingsLayoutProps {
  readonly tabs: Tab[];
  readonly renderTab: (tabId: string) => ReactNode;
}

export function SettingsLayout({ tabs, renderTab }: SettingsLayoutProps) {
  const searchParams = useSearchParams();
  const requestedTab = searchParams.get("tab");
  const requestedTabId = requestedTab ? `settings-${requestedTab}` : null;
  const initialTab =
    requestedTabId && tabs.some((tab) => tab.id === requestedTabId)
      ? requestedTabId
      : (tabs[0]?.id ?? null);
  const [activeTab, setActiveTab] = useState<string | null>(initialTab);
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

  useEffect(() => {
    if (!requestedTabId) return;
    if (!tabs.some((tab) => tab.id === requestedTabId)) return;
    setActiveTab(requestedTabId);
  }, [requestedTabId, tabs]);

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
            <div className="pb-6">
              <MobileTabList tabs={tabs} onSelect={(id) => setActiveTab(id)} />
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
