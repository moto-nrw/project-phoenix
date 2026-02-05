"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { PasswordChangeModal } from "~/components/ui";
import { Loading } from "~/components/ui/loading";
import { navigationIcons } from "~/lib/navigation-icons";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useOperatorAuth } from "~/lib/operator/auth-context";

interface ProfileResponse {
  data: {
    id: number;
    email: string;
    display_name: string;
  };
}

// Tab configuration — identical structure to teacher settings
interface Tab {
  id: string;
  label: string;
  icon: string;
}

const tabs: Tab[] = [
  {
    id: "profile",
    label: "Profil",
    icon: navigationIcons.profile,
  },
  {
    id: "security",
    label: "Sicherheit",
    icon: navigationIcons.security,
  },
];

function OperatorSettingsContent() {
  const {
    operator,
    isLoading: authLoading,
    updateOperator,
  } = useOperatorAuth();
  const [activeTab, setActiveTab] = useState<string | null>("profile");
  const [showAlert, setShowAlert] = useState(false);
  const [alertMessage, setAlertMessage] = useState("");
  const [alertType, setAlertType] = useState<"success" | "error">("success");
  const [isMobile, setIsMobile] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  // Profile editing state
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    displayName: "",
    email: "",
  });

  // Memoized callback for alert close to prevent re-renders from resetting timer
  const handleAlertClose = useCallback(() => {
    setShowAlert(false);
  }, []);

  // Handle back navigation on mobile
  const handleBackToList = useCallback(() => {
    setActiveTab(null);
  }, []);

  // Handle tab selection
  const handleTabSelect = useCallback((tabId: string) => {
    setActiveTab(tabId);
  }, []);

  useEffect(() => {
    const handleResize = () => {
      const wasDesktop = !isMobile;
      const isNowMobile = window.innerWidth < 768;
      setIsMobile(isNowMobile);

      // Reset to list view when switching to mobile
      if (wasDesktop && isNowMobile) {
        setActiveTab(null);
      }
      // Set profile tab when switching to desktop
      else if (!wasDesktop && !isNowMobile && activeTab === null) {
        setActiveTab("profile");
      }
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile, activeTab]);

  // Sync formData with operator from Context
  useEffect(() => {
    if (operator) {
      setFormData({
        displayName: operator.displayName || "",
        email: operator.email || "",
      });
    }
  }, [operator]);

  const handleSaveProfile = async () => {
    setIsSaving(true);
    try {
      const response = await fetch("/api/operator/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ display_name: formData.displayName }),
      });

      if (!response.ok) {
        const data = (await response.json()) as { error?: string };
        throw new Error(
          data.error ?? "Profil konnte nicht aktualisiert werden",
        );
      }

      const result = (await response.json()) as ProfileResponse;
      updateOperator({ displayName: result.data.display_name.toString() });

      setIsEditing(false);
      setAlertMessage("Profil erfolgreich aktualisiert");
      setAlertType("success");
      setShowAlert(true);
    } catch {
      setAlertMessage("Fehler beim Speichern des Profils");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  if (authLoading) {
    return <Loading fullPage={false} />;
  }

  const renderTabContent = () => {
    switch (activeTab) {
      case "profile":
        return (
          <div className="space-y-6">
            {/* Initials Avatar — operators have no uploaded avatar */}
            <div className={`mb-8 ${isMobile ? "flex justify-center" : ""}`}>
              <div className="inline-block">
                <div className="flex flex-col items-center">
                  <div className="relative flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white shadow-xl">
                    <span className="text-4xl font-bold">
                      {formData.displayName
                        .split(" ")
                        .map((p) => p.charAt(0))
                        .join("")
                        .slice(0, 2)
                        .toUpperCase()}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Profile Form */}
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor="settings-displayname"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    Anzeigename
                  </label>
                  <input
                    id="settings-displayname"
                    type="text"
                    value={formData.displayName}
                    onChange={(e) =>
                      setFormData({ ...formData, displayName: e.target.value })
                    }
                    disabled={!isEditing}
                    className="w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
                  />
                </div>
                <div>
                  <label
                    htmlFor="settings-email"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    E-Mail
                  </label>
                  <input
                    id="settings-email"
                    type="email"
                    value={formData.email}
                    disabled
                    className="w-full rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-base text-gray-500"
                  />
                </div>
              </div>
            </div>

            {/* Action Buttons */}
            <div className={`flex gap-3 ${isMobile ? "justify-center" : ""}`}>
              {isEditing ? (
                <>
                  <button
                    onClick={() => {
                      setIsEditing(false);
                      if (operator) {
                        setFormData({
                          displayName: operator.displayName || "",
                          email: operator.email || "",
                        });
                      }
                    }}
                    className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
                  >
                    Abbrechen
                  </button>
                  <button
                    onClick={() => void handleSaveProfile()}
                    disabled={isSaving}
                    className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:opacity-50 disabled:hover:scale-100"
                  >
                    {isSaving ? "Speichern..." : "Speichern"}
                  </button>
                </>
              ) : (
                <button
                  onClick={() => setIsEditing(true)}
                  className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
                >
                  Bearbeiten
                </button>
              )}
            </div>
          </div>
        );

      case "security":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                Passwort ändern
              </h3>
              <p className="mb-4 text-sm text-gray-600">
                Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche
                Sicherheit.
              </p>
              <button
                onClick={() => setShowPasswordModal(true)}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
              >
                Passwort ändern
              </button>
            </div>
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="-mt-1.5 w-full">
      {/* Header - Only show on mobile list view */}
      {isMobile && activeTab === null && (
        <PageHeaderWithSearch title="Einstellungen" />
      )}

      {/* Tab Navigation - Desktop with line variant */}
      {!isMobile && (
        <div className="mb-6 ml-6">
          <Tabs value={activeTab ?? "profile"} onValueChange={setActiveTab}>
            <TabsList variant="line">
              {tabs.map((tab) => (
                <TabsTrigger key={tab.id} value={tab.id}>
                  {tab.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      )}

      {/* Mobile Navigation */}
      {isMobile && (
        <>
          {/* Mobile List View - iOS Style */}
          {activeTab === null ? (
            <div className="flex flex-col space-y-6 pb-6">
              {/* Profile Card - iOS Apple ID Style */}
              <div className="mx-4">
                <button
                  onClick={() => handleTabSelect("profile")}
                  className="flex w-full items-center gap-4 rounded-2xl bg-white p-4 shadow-sm transition-colors hover:bg-gray-50 active:bg-gray-100"
                >
                  {/* Initials Avatar */}
                  <div className="relative flex h-16 w-16 flex-shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white">
                    <span className="text-xl font-bold">
                      {formData.displayName
                        .split(" ")
                        .map((p) => p.charAt(0))
                        .join("")
                        .slice(0, 2)
                        .toUpperCase()}
                    </span>
                  </div>
                  {/* Name & Subtitle */}
                  <div className="flex-1 text-left">
                    <p className="text-lg font-semibold text-gray-900">
                      {formData.displayName}
                    </p>
                    <p className="text-sm text-gray-500">Profil, Anzeigename</p>
                  </div>
                  {/* Chevron */}
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

              {/* Security Settings Section */}
              <div className="space-y-2">
                <div className="mx-4 overflow-hidden rounded-2xl bg-white shadow-sm">
                  <button
                    onClick={() => handleTabSelect("security")}
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
            </div>
          ) : (
            /* Mobile Detail View */
            <div className="flex h-[calc(100vh-120px)] flex-col">
              {/* Mobile Header with Back Button */}
              <div className="mb-4 flex items-center gap-3 pb-4">
                <button
                  onClick={handleBackToList}
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
                  {tabs.find((t) => t.id === activeTab)?.label}
                </h2>
              </div>
              {/* Scrollable Content */}
              <div className="-mx-4 flex-1 overflow-y-auto px-4">
                {renderTabContent()}
              </div>
            </div>
          )}
        </>
      )}

      {/* Desktop Content */}
      {!isMobile && <div className="min-h-[60vh]">{renderTabContent()}</div>}

      {/* Alerts */}
      {showAlert && (
        <SimpleAlert
          type={alertType}
          message={alertMessage}
          onClose={handleAlertClose}
          autoClose
          duration={3000}
        />
      )}

      {/* Password Change Modal */}
      {showPasswordModal && (
        <PasswordChangeModal
          isOpen={showPasswordModal}
          onClose={() => setShowPasswordModal(false)}
          apiEndpoint="/api/operator/profile/password"
          onSuccess={() => {
            setShowPasswordModal(false);
            setAlertMessage("Passwort erfolgreich geändert");
            setAlertType("success");
            setShowAlert(true);
          }}
        />
      )}
    </div>
  );
}

export default function OperatorSettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <OperatorSettingsContent />
    </Suspense>
  );
}
