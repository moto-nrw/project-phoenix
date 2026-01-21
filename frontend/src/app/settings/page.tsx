"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "~/lib/auth-client";
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

// Tab configuration
interface Tab {
  id: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
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

const adminTabs: Tab[] = [];

function SettingsContent() {
  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();
  const { success: toastSuccess } = useToast();
  const { profile, updateProfileData, refreshProfile } = useProfile();
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
    firstName: "",
    lastName: "",
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
    // BetterAuth: cookies handle auth, no token check needed
    if (!session?.user || !profile) return;

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
    // BetterAuth: cookies handle auth, no token check needed
    if (!session?.user) return;

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

  if (isPending) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (!session?.user) {
    redirect("/");
  }

  // BetterAuth: Admin status would need to be fetched via getActiveRole()
  // For now, check if user object has isAdmin property (backward compatibility)
  const isUserAdmin =
    session?.user &&
    "isAdmin" in session.user &&
    (session.user as { isAdmin?: boolean }).isAdmin;
  const allTabs = isUserAdmin ? [...tabs, ...adminTabs] : tabs;

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
        return null;
    }
  };

  return (
    <ResponsiveLayout>
      <div className="-mt-1.5 w-full">
        {/* Header - Only show on mobile list view */}
        {isMobile && activeTab === null && (
          <PageHeaderWithSearch title="Einstellungen" />
        )}

        {/* Tab Navigation - Desktop with new underline style */}
        {!isMobile && (
          <div className="mb-6 ml-6">
            <div className="relative flex gap-8">
              {allTabs.map((tab) => {
                const isActive = activeTab === tab.id;
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
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
                        d={tab.icon}
                      />
                    </svg>
                    <span>{tab.label}</span>
                    {tab.adminOnly && (
                      <span className="ml-1 rounded bg-gray-200 px-1.5 py-0.5 text-xs">
                        Admin
                      </span>
                    )}
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
                    {allTabs.find((t) => t.id === activeTab)?.label}
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
