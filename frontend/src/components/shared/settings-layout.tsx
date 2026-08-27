"use client";

import {
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from "react";
import { useSearchParams } from "next/navigation";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "~/components/ui/button";
import { PageIntro } from "~/components/ui/page-intro";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

interface Tab {
  id: string;
  label: string;
  icon: MotoConceptKey;
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
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={onBack}
        className="-ml-2"
        aria-label="Zurück"
      >
        <ChevronLeft className="h-5 w-5 text-gray-900" />
      </Button>
      <h2 className="text-base font-semibold text-gray-900">
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
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      {tabs.map((tab, idx) => (
        <button
          type="button"
          key={tab.id}
          onClick={() => onSelect(tab.id)}
          className={`flex w-full items-center gap-3 px-4 py-3.5 text-left transition-colors active:bg-gray-50 ${
            idx < tabs.length - 1 ? "border-b border-gray-100" : ""
          }`}
        >
          <MotoConceptIcon concept={tab.icon} size={18} className="shrink-0" />
          <span className="flex-1 text-[15px] text-gray-900">{tab.label}</span>
          {tab.adminOnly && (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500">
              Admin
            </span>
          )}
          <ChevronRight className="h-4 w-4 shrink-0 text-gray-300" />
        </button>
      ))}
    </div>
  );
}

/**
 * Desktop tab bar: kit Tabs (line variant) in a horizontal scroll container
 * with fade indicators at the edges when tabs overflow, and the active tab
 * kept scrolled into view.
 */
function DesktopSettingsTabs({
  tabs,
  activeTab,
  onTabChange,
  className = "",
}: {
  readonly tabs: Tab[];
  readonly activeTab: string;
  readonly onTabChange: (id: string) => void;
  readonly className?: string;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 0);
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
  }, []);

  useEffect(() => {
    updateScrollState();
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener("scroll", updateScrollState, { passive: true });
    const observer = new ResizeObserver(updateScrollState);
    observer.observe(el);
    return () => {
      el.removeEventListener("scroll", updateScrollState);
      observer.disconnect();
    };
  }, [updateScrollState, tabs]);

  // Keep the active tab fully visible when it changes (e.g. deep link).
  useEffect(() => {
    const el = scrollRef.current;
    const tabEl = el?.querySelector<HTMLElement>(`[data-state="active"]`);
    if (!el || !tabEl) return;
    const tabLeft = tabEl.offsetLeft;
    const tabRight = tabLeft + tabEl.offsetWidth;
    if (tabLeft < el.scrollLeft) {
      el.scrollTo({ left: tabLeft - 16, behavior: "smooth" });
    } else if (tabRight > el.scrollLeft + el.clientWidth) {
      el.scrollTo({
        left: tabRight - el.clientWidth + 16,
        behavior: "smooth",
      });
    }
  }, [activeTab]);

  return (
    <div className={`relative ${className}`}>
      {canScrollLeft && (
        <div className="pointer-events-none absolute top-0 bottom-0 left-0 z-10 w-8 bg-gradient-to-r from-white to-transparent" />
      )}
      <div ref={scrollRef} className="scrollbar-hidden overflow-x-auto">
        <Tabs value={activeTab} onValueChange={onTabChange}>
          <TabsList variant="line" className="w-max min-w-full justify-start">
            {tabs.map((tab) => (
              <TabsTrigger key={tab.id} value={tab.id}>
                <MotoConceptIcon concept={tab.icon} size={16} />
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
      {canScrollRight && (
        <div className="pointer-events-none absolute top-0 right-0 bottom-0 z-10 w-8 bg-gradient-to-l from-white to-transparent" />
      )}
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
    <div className="w-full">
      {/* Kopfkarte auf allen Breakpoints, wie in der Eltern-App. In der
          mobilen Detailansicht trägt der Zurück-Kopf den Reiternamen, dort
          wäre die Kopfkarte ein zweiter Kopf. */}
      {/* Auf dem Desktop tragen Kopfkarte und Reiterleiste EINE Zeile
          zusammen, statt zwei fast leere Zeilen zu stapeln. */}
      {!(isMobile && activeTab !== null) && (
        <PageIntro
          title="Einstellungen"
          description={`${tabs.length} ${tabs.length === 1 ? "Bereich" : "Bereiche"}`}
          className="mb-6"
        >
          {/* Bewusst `undefined` statt `false` im Mobilfall: die Kopfkarte
              rendert ihren Rumpf, sobald children nicht null/undefined ist —
              ein `false` erzeugte mobil einen leeren mt-4-Block unter der
              Statuszeile. */}
          {!isMobile ? (
            <DesktopSettingsTabs
              tabs={tabs}
              activeTab={activeTab ?? tabs[0]?.id ?? ""}
              onTabChange={setActiveTab}
              className="mt-3 -mb-2"
            />
          ) : undefined}
        </PageIntro>
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
