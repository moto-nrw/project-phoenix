"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Image from "next/image";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { useToast } from "~/contexts/ToastContext";
import { PasswordChangeModal } from "~/components/ui";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { Loading } from "~/components/ui/loading";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { navigationIcons } from "~/lib/navigation-icons";
import {
  clientFetchSettingsTabs,
  clientFetchTabSettings,
  clientFetchSettingValue,
} from "~/lib/settings-api";
import type { SettingTab, TabSettingsResponse, Scope } from "~/lib/settings-helpers";
import { SettingsCategory } from "~/components/settings/settings-category";

// Hardcoded tabs that have special UI (not just settings)
interface HardcodedTab {
  id: string;
  label: string;
  icon: string;
  isHardcoded: true;
}

const hardcodedTabs: HardcodedTab[] = [
  {
    id: "profile",
    label: "Profil",
    icon: navigationIcons.profile,
    isHardcoded: true,
  },
  {
    id: "security",
    label: "Sicherheit",
    icon: navigationIcons.security,
    isHardcoded: true,
  },
];

// Icon mapping for backend tabs
const tabIconMap: Record<string, string> = {
  general: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
  email: "M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
  display: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
  notifications: "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9",
  rooms: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
  system: "M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01",
  test: "M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z",
};

// Combined tab type
type Tab = HardcodedTab | (SettingTab & { isHardcoded?: false });

function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const { success: toastSuccess } = useToast();
  const { profile, updateProfileData, refreshProfile } = useProfile();
  const [activeTab, setActiveTab] = useState<string | null>("profile");
  const [showAlert, setShowAlert] = useState(false);
  const [alertMessage, setAlertMessage] = useState("");
  const [alertType, setAlertType] = useState<"success" | "error">("success");
  const [isMobile, setIsMobile] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  // Backend settings tabs
  const [backendTabs, setBackendTabs] = useState<SettingTab[]>([]);
  const [tabSettings, setTabSettings] = useState<TabSettingsResponse | null>(null);
  const [tabLoading, setTabLoading] = useState(false);

  // Profile editing state
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    email: "",
  });

  // Welcome banner state (demo for settings system)
  const [showWelcomeBanner, setShowWelcomeBanner] = useState<boolean | null>(null);

  // Load welcome banner setting
  useEffect(() => {
    let isMounted = true;

    async function loadWelcomeBannerSetting() {
      const setting = await clientFetchSettingValue("display.show_welcome_banner");
      if (isMounted && setting) {
        setShowWelcomeBanner(setting.effectiveValue === "true");
      }
    }

    void loadWelcomeBannerSetting();

    return () => {
      isMounted = false;
    };
  }, []);

  // Load backend tabs on mount
  useEffect(() => {
    let isMounted = true;

    async function loadBackendTabs() {
      const tabs = await clientFetchSettingsTabs();
      if (isMounted) {
        setBackendTabs(tabs);
      }
    }

    void loadBackendTabs();

    return () => {
      isMounted = false;
    };
  }, []);

  // Load tab settings when selecting a backend tab
  useEffect(() => {
    if (!activeTab) return;

    // Only load settings for backend tabs (not profile/security)
    const isBackendTab = backendTabs.some((t) => t.key === activeTab);
    if (!isBackendTab) {
      setTabSettings(null);
      return;
    }

    let isMounted = true;

    async function loadTabSettings() {
      setTabLoading(true);
      try {
        const settings = await clientFetchTabSettings(activeTab!);
        if (isMounted) {
          setTabSettings(settings);
        }
      } catch (err) {
        console.error("Error loading tab settings:", err);
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
  }, [activeTab, backendTabs]);

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

  // Sync formData with profile from Context
  useEffect(() => {
    if (profile) {
      setFormData({
        firstName: profile.firstName || "",
        lastName: profile.lastName || "",
        email: profile.email || "",
      });
    }
  }, [profile]);

  const handleSaveProfile = async () => {
    if (!session?.user?.token || !profile) return;

    setIsSaving(true);
    try {
      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
      };

      await updateProfile(updateData);

      // Update profile in Context (optimistic update)
      updateProfileData({
        firstName: formData.firstName,
        lastName: formData.lastName,
      });

      // Refresh from backend to ensure consistency
      await refreshProfile(true);

      setIsEditing(false);
      toastSuccess("Profil erfolgreich aktualisiert");
    } catch {
      setAlertMessage("Fehler beim Speichern des Profils");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  const handleAvatarChange = async (file: File) => {
    if (!session?.user?.token) return;

    try {
      setIsSaving(true);

      // 1. Compress image in browser before upload
      const compressedFile = await compressAvatar(file);

      // 2. Upload compressed file (much faster!)
      await uploadAvatar(compressedFile);

      // 3. Refresh profile from backend to get new avatar URL
      await refreshProfile(true);

      toastSuccess("Profilbild erfolgreich aktualisiert");
    } catch {
      setAlertMessage("Fehler beim Hochladen des Profilbilds");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  const handleSettingChanged = useCallback(() => {
    // Refresh tab settings after a change
    if (activeTab && backendTabs.some((t) => t.key === activeTab)) {
      void clientFetchTabSettings(activeTab).then(setTabSettings);
    }
    // Also refresh the welcome banner setting
    void clientFetchSettingValue("display.show_welcome_banner").then((setting) => {
      if (setting) {
        setShowWelcomeBanner(setting.effectiveValue === "true");
      }
    });
  }, [activeTab, backendTabs]);

  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (!session?.user) {
    redirect("/");
  }

  // Combine hardcoded tabs with backend tabs (filter out duplicates)
  const hardcodedTabIds = new Set(hardcodedTabs.map((t) => t.id));
  const allTabs: Tab[] = [
    ...hardcodedTabs,
    ...backendTabs
      .filter((t) => !hardcodedTabIds.has(t.key)) // Exclude backend tabs that conflict with hardcoded ones
      .map((t) => ({ ...t, isHardcoded: false as const })),
  ];

  // Default icon path (cog/gear)
  const defaultIconPath = "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z";

  // Get icon for a tab
  const getTabIcon = (tab: Tab): string => {
    if ("icon" in tab && typeof tab.icon === "string" && tab.icon.length > 20) {
      // It's an SVG path
      return tab.icon;
    }
    // Try to get from icon map
    const key = "key" in tab ? tab.key : tab.id;
    return tabIconMap[key] ?? defaultIconPath;
  };

  // Get label for a tab
  const getTabLabel = (tab: Tab): string => {
    if ("label" in tab && tab.label) return tab.label;
    if ("name" in tab && tab.name) return tab.name;
    // Fallback to key or id
    if ("key" in tab) return tab.key;
    return tab.id;
  };

  // Get ID for a tab
  const getTabId = (tab: Tab): string => {
    if ("id" in tab) return tab.id;
    if ("key" in tab) return tab.key;
    return "";
  };

  const renderTabContent = () => {
    switch (activeTab) {
      case "profile":
        return (
          <div className="space-y-6">
            {/* Avatar Section - Center on mobile */}
            <div className={`mb-8 ${isMobile ? "flex justify-center" : ""}`}>
              <div className="inline-block">
                <div className="flex flex-col items-center">
                  <div className="group relative">
                    <div className="relative flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white shadow-xl">
                      {profile?.avatar ? (
                        <Image
                          src={profile.avatar}
                          alt="Profile"
                          fill
                          className="object-cover"
                          sizes="128px"
                          priority
                          unoptimized
                        />
                      ) : (
                        <span className="text-4xl font-bold">
                          {(formData.firstName?.charAt(0) || "") +
                            (formData.lastName?.charAt(0) || "")}
                        </span>
                      )}
                    </div>
                    <label
                      htmlFor="avatar-upload"
                      aria-label="Profilbild ändern"
                      className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
                    >
                      <svg
                        className="h-8 w-8 text-white"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                        />
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
                        />
                      </svg>
                    </label>
                    <input
                      id="avatar-upload"
                      type="file"
                      accept="image/*"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) void handleAvatarChange(file);
                      }}
                      className="hidden"
                    />
                  </div>
                  <button
                    type="button"
                    onClick={() =>
                      document.getElementById("avatar-upload")?.click()
                    }
                    className="mt-3 text-[11px] font-medium text-gray-600 transition-colors hover:text-gray-900"
                  >
                    Profilbild ändern
                  </button>
                </div>
              </div>
            </div>

            {/* Profile Form - Better mobile layout */}
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor="settings-firstname"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    Vorname
                  </label>
                  <input
                    id="settings-firstname"
                    type="text"
                    value={formData.firstName}
                    onChange={(e) =>
                      setFormData({ ...formData, firstName: e.target.value })
                    }
                    disabled={!isEditing}
                    className="w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
                  />
                </div>
                <div>
                  <label
                    htmlFor="settings-lastname"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    Nachname
                  </label>
                  <input
                    id="settings-lastname"
                    type="text"
                    value={formData.lastName}
                    onChange={(e) =>
                      setFormData({ ...formData, lastName: e.target.value })
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
                      // Reset form data to profile values from Context
                      if (profile) {
                        setFormData({
                          firstName: profile.firstName || "",
                          lastName: profile.lastName || "",
                          email: profile.email || "",
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
        // Backend settings tab
        if (tabLoading) {
          return (
            <div className="flex justify-center py-8">
              <Loading fullPage={false} />
            </div>
          );
        }

        if (!tabSettings) {
          return (
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
              <p className="text-sm text-gray-600">
                Keine Einstellungen verfügbar.
              </p>
            </div>
          );
        }

        const scope: Scope = "system";

        return (
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
        );
    }
  };

  return (
    <ResponsiveLayout>
      <div className="-mt-1.5 w-full">
        {/* Welcome Banner - controlled by display.show_welcome_banner setting */}
        {showWelcomeBanner && (
          <div className="mx-4 mb-4 rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 p-4 text-white shadow-md md:mx-6">
            <div className="flex items-center gap-3">
              <svg
                className="h-6 w-6 flex-shrink-0"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <div>
                <p className="font-medium">Willkommen bei den Einstellungen!</p>
                <p className="text-sm text-blue-100">
                  Dieses Banner wird durch die Einstellung &quot;Willkommensbanner anzeigen&quot; im Tab &quot;Anzeige&quot; gesteuert.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Header - Only show on mobile list view */}
        {isMobile && activeTab === null && (
          <PageHeaderWithSearch title="Einstellungen" />
        )}

        {/* Tab Navigation - Desktop with new underline style */}
        {!isMobile && (
          <div className="mb-6 ml-6">
            <div className="relative flex flex-wrap gap-6">
              {allTabs.map((tab) => {
                const tabId = getTabId(tab);
                const isActive = activeTab === tabId;
                return (
                  <button
                    key={tabId}
                    onClick={() => setActiveTab(tabId)}
                    className={`relative flex items-center gap-2 pb-3 text-sm font-medium transition-all ${
                      isActive
                        ? "font-semibold text-gray-900"
                        : "text-gray-500 hover:text-gray-700"
                    } `}
                  >
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
                        d={getTabIcon(tab)}
                      />
                    </svg>
                    <span>{getTabLabel(tab)}</span>
                    {isActive && (
                      <div className="absolute right-0 bottom-0 left-0 h-0.5 rounded-full bg-gray-900" />
                    )}
                  </button>
                );
              })}
            </div>
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
                    {/* Avatar */}
                    <div className="relative flex h-16 w-16 flex-shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white">
                      {profile?.avatar ? (
                        <Image
                          src={profile.avatar}
                          alt="Profile"
                          fill
                          className="object-cover"
                          sizes="64px"
                          unoptimized
                        />
                      ) : (
                        <span className="text-xl font-bold">
                          {(formData.firstName?.charAt(0) || "") +
                            (formData.lastName?.charAt(0) || "")}
                        </span>
                      )}
                    </div>
                    {/* Name & Subtitle */}
                    <div className="flex-1 text-left">
                      <p className="text-lg font-semibold text-gray-900">
                        {formData.firstName} {formData.lastName}
                      </p>
                      <p className="text-sm text-gray-500">
                        Profil, Profilbild
                      </p>
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

                {/* Other tabs */}
                <div className="space-y-2">
                  <div className="mx-4 overflow-hidden rounded-2xl bg-white shadow-sm">
                    {allTabs
                      .filter((tab) => getTabId(tab) !== "profile")
                      .map((tab, index, arr) => {
                        const tabId = getTabId(tab);
                        return (
                          <button
                            key={tabId}
                            onClick={() => handleTabSelect(tabId)}
                            className={`flex w-full items-center justify-between px-4 py-3.5 transition-colors hover:bg-gray-50 active:bg-gray-100 ${
                              index < arr.length - 1 ? "border-b border-gray-100" : ""
                            }`}
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
                                    d={getTabIcon(tab)}
                                  />
                                </svg>
                              </div>
                              <p className="text-base text-gray-900">
                                {getTabLabel(tab)}
                              </p>
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
                        );
                      })}
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
                    {allTabs.find((t) => getTabId(t) === activeTab)
                      ? getTabLabel(allTabs.find((t) => getTabId(t) === activeTab)!)
                      : ""}
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
        {showAlert && alertType !== "success" && (
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
            onSuccess={() => {
              setShowPasswordModal(false);
              toastSuccess("Passwort erfolgreich geändert");
            }}
          />
        )}
      </div>
    </ResponsiveLayout>
  );
}

export default function SettingsPage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <SettingsContent />
    </Suspense>
  );
}
