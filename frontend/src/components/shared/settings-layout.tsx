"use client";

import { useState, useEffect, useCallback, type ReactNode } from "react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { PasswordChangeModal } from "~/components/ui";
import { navigationIcons } from "~/lib/navigation-icons";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";

interface Tab {
  id: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
}

const baseTabs: Tab[] = [
  { id: "profile", label: "Profil", icon: navigationIcons.profile },
  { id: "security", label: "Sicherheit", icon: navigationIcons.security },
];

interface MobileProfileCardProps {
  readonly onSelect: () => void;
  readonly children: ReactNode;
}

function MobileProfileCard({ onSelect, children }: MobileProfileCardProps) {
  return (
    <div className="mx-4">
      <button
        onClick={onSelect}
        className="flex w-full items-center gap-4 rounded-2xl bg-white p-4 shadow-sm transition-colors hover:bg-gray-50 active:bg-gray-100"
      >
        {children}
        <svg
          className="h-5 w-5 flex-shrink-0 text-gray-400"
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

function MobileSecurityButton({ onSelect }: { readonly onSelect: () => void }) {
  return (
    <div className="space-y-2">
      <div className="mx-4 overflow-hidden rounded-2xl bg-white shadow-sm">
        <button
          onClick={onSelect}
          className="flex w-full items-center justify-between px-4 py-3.5 transition-colors hover:bg-gray-50 active:bg-gray-100"
        >
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100">
              <svg
                className="h-4 w-4 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d={navigationIcons.security}
                />
              </svg>
            </div>
            <p className="text-base text-gray-900">Sicherheit</p>
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
    </div>
  );
}

function SecurityTabContent({
  onChangePassword,
}: {
  readonly onChangePassword: () => void;
}) {
  return (
    <div className="space-y-6">
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
        <h3 className="mb-3 text-base font-semibold text-gray-900">
          Passwort ändern
        </h3>
        <p className="mb-4 text-sm text-gray-600">
          Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit.
        </p>
        <button
          onClick={onChangePassword}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
        >
          Passwort ändern
        </button>
      </div>
    </div>
  );
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
        className="-ml-2 rounded-lg p-2 transition-all hover:bg-gray-100 active:bg-gray-200"
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

export interface SettingsLayoutProps {
  readonly profileTab: ReactNode;
  readonly mobileProfileCard: ReactNode;
  readonly extraTabs?: Tab[];
  readonly passwordApiEndpoint?: string;
  readonly onPasswordSuccess?: () => void;
  readonly alert?: {
    show: boolean;
    type: "success" | "error";
    message: string;
    onClose: () => void;
  };
}

export function SettingsLayout({
  profileTab,
  mobileProfileCard,
  extraTabs,
  passwordApiEndpoint,
  onPasswordSuccess,
  alert,
}: SettingsLayoutProps) {
  const [activeTab, setActiveTab] = useState<string | null>("profile");
  const [isMobile, setIsMobile] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  const handleBackToList = useCallback(() => {
    setActiveTab(null);
  }, []);

  const handleTabSelect = useCallback((tabId: string) => {
    setActiveTab(tabId);
  }, []);

  useEffect(() => {
    const handleResize = () => {
      const wasDesktop = !isMobile;
      const isNowMobile = window.innerWidth < 768;
      setIsMobile(isNowMobile);

      if (wasDesktop && isNowMobile) {
        setActiveTab(null);
      } else if (!wasDesktop && !isNowMobile && activeTab === null) {
        setActiveTab("profile");
      }
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile, activeTab]);

  const allTabs = extraTabs ? [...baseTabs, ...extraTabs] : baseTabs;

  const renderTabContent = () => {
    switch (activeTab) {
      case "profile":
        return profileTab;
      case "security":
        return (
          <SecurityTabContent
            onChangePassword={() => setShowPasswordModal(true)}
          />
        );
      default:
        return null;
    }
  };

  return (
    <div className="-mt-1.5 w-full">
      {isMobile && activeTab === null && (
        <PageHeaderWithSearch title="Einstellungen" />
      )}

      {!isMobile && (
        <div className="mb-6 ml-6">
          <Tabs value={activeTab ?? "profile"} onValueChange={setActiveTab}>
            <TabsList variant="line">
              {allTabs.map((tab) => (
                <TabsTrigger key={tab.id} value={tab.id}>
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
            <div className="flex flex-col space-y-6 pb-6">
              <MobileProfileCard onSelect={() => handleTabSelect("profile")}>
                {mobileProfileCard}
              </MobileProfileCard>
              <MobileSecurityButton
                onSelect={() => handleTabSelect("security")}
              />
            </div>
          ) : (
            <div className="flex h-[calc(100vh-120px)] flex-col">
              <MobileBackHeader
                onBack={handleBackToList}
                tabs={allTabs}
                activeTab={activeTab}
              />
              <div className="-mx-4 flex-1 overflow-y-auto px-4">
                {renderTabContent()}
              </div>
            </div>
          )}
        </>
      )}

      {!isMobile && <div className="min-h-[60vh]">{renderTabContent()}</div>}

      {alert?.show && (
        <SimpleAlert
          type={alert.type}
          message={alert.message}
          onClose={alert.onClose}
          autoClose
          duration={3000}
        />
      )}

      {showPasswordModal && (
        <PasswordChangeModal
          isOpen={showPasswordModal}
          onClose={() => setShowPasswordModal(false)}
          apiEndpoint={passwordApiEndpoint}
          onSuccess={() => {
            setShowPasswordModal(false);
            onPasswordSuccess?.();
          }}
        />
      )}
    </div>
  );
}
