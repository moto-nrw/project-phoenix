"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import Image from "next/image";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { useToast } from "~/contexts/ToastContext";
import { PasswordChangeModal } from "~/components/ui";
import { PINManagement } from "~/components/staff";
import { IOSToggle } from "~/components/ui/ios-toggle";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { Loading } from "~/components/ui/loading";
import { useProfile } from "~/lib/profile-context";

// Tab configuration
interface Tab {
  id: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
  disabled?: boolean;
}

const tabs: Tab[] = [
  {
    id: "profile",
    label: "Profil",
    icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
  },
  {
    id: "security",
    label: "Sicherheit",
    icon: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
    disabled: true,
  },
  {
    id: "notifications",
    label: "Benachrichtigungen",
    icon: "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9",
    disabled: true,
  },
  {
    id: "appearance",
    label: "Darstellung",
    icon: "M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z",
    disabled: true,
  },
  {
    id: "privacy",
    label: "Privatsphäre",
    icon: "M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z",
    disabled: true,
  },
];

const adminTabs: Tab[] = [
  {
    id: "data",
    label: "Datenverwaltung",
    icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4",
    adminOnly: true,
    disabled: true,
  },
  {
    id: "subscription",
    label: "Abonnement",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01",
    adminOnly: true,
    disabled: true,
  },
  {
    id: "payment",
    label: "Zahlungsmethode",
    icon: "M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z",
    adminOnly: true,
    disabled: true,
  },
];

function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const router = useRouter();
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

  // Settings state
  const [emailNotifications, setEmailNotifications] = useState(true);
  const [pushNotifications, setPushNotifications] = useState(false);
  const [activityUpdates, setActivityUpdates] = useState(true);
  const [darkMode, setDarkMode] = useState(false);
  const [dataSharing, setDataSharing] = useState(false);
  const [activityTracking, setActivityTracking] = useState(true);
  const [emailChannel, setEmailChannel] = useState(true);
  const [browserChannel, setBrowserChannel] = useState(false);

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
      await uploadAvatar(file);

      // Refresh profile from backend to get new avatar URL
      await refreshProfile(true);

      toastSuccess("Profilbild erfolgreich aktualisiert");
    } catch {
      setAlertMessage("Fehler beim Hochladen des Profilbilds");
      setAlertType("error");
      setShowAlert(true);
    }
  };

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

  const allTabs = session?.user?.isAdmin ? [...tabs, ...adminTabs] : tabs;

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
                  <label className="mb-2 block text-sm font-medium text-gray-700">
                    Vorname
                  </label>
                  <input
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
                  <label className="mb-2 block text-sm font-medium text-gray-700">
                    Nachname
                  </label>
                  <input
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
                  <label className="mb-2 block text-sm font-medium text-gray-700">
                    E-Mail
                  </label>
                  <input
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

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                PIN-Verwaltung
              </h3>
              <p className="mb-4 text-sm text-gray-600">
                Verwalten Sie Ihre PIN für RFID-Geräte.
              </p>
              <PINManagement
                onSuccess={(message) => {
                  toastSuccess(message);
                }}
                onError={(message) => {
                  setAlertMessage(message);
                  setAlertType("error");
                  setShowAlert(true);
                }}
              />
            </div>
          </div>
        );

      case "notifications":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Kritische Benachrichtigungen
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      Notfallmeldungen
                    </p>
                    <p className="text-xs text-gray-500">
                      Sofortige Benachrichtigung bei Notfällen
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={true}
                      onChange={() => {
                        // Disabled toggle - no action needed
                      }}
                      disabled={true}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      Nicht abgeholte Kinder
                    </p>
                    <p className="text-xs text-gray-500">
                      Erinnerung wenn Kinder nach Schließzeit noch da sind
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={emailNotifications}
                      onChange={setEmailNotifications}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      Vertretungsanfragen
                    </p>
                    <p className="text-xs text-gray-500">
                      Benachrichtigung bei Vertretungsbedarf
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={activityUpdates}
                      onChange={setActivityUpdates}
                    />
                  </div>
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Wöchentlicher Statusbericht
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      OGS-Betriebsübersicht
                    </p>
                    <p className="text-xs text-gray-500">
                      Zusammenfassung: Anwesenheit, Aktivitäten, Vorfälle
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={pushNotifications}
                      onChange={setPushNotifications}
                    />
                  </div>
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Benachrichtigungskanal
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      E-Mail-Benachrichtigungen
                    </p>
                    <p className="text-xs text-gray-500">
                      Benachrichtigungen per E-Mail erhalten
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={emailChannel}
                      onChange={setEmailChannel}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      Browser-Benachrichtigungen
                    </p>
                    <p className="text-xs text-gray-500">
                      Push-Benachrichtigungen im Browser
                    </p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={browserChannel}
                      onChange={setBrowserChannel}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        );

      case "appearance":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Design
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      Dunkler Modus
                    </p>
                    <p className="text-xs text-gray-500">
                      Augenschonendes Design für dunklere Umgebungen
                    </p>
                  </div>
                  <IOSToggle checked={darkMode} onChange={setDarkMode} />
                </div>
              </div>
            </div>
          </div>
        );

      case "privacy":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Datenschutz
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      Daten mit moto teilen
                    </p>
                    <p className="text-xs text-gray-500">
                      Ihre Daten werden anonymisiert geteilt
                    </p>
                  </div>
                  <IOSToggle checked={dataSharing} onChange={setDataSharing} />
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      Fehlerberichte senden
                    </p>
                    <p className="text-xs text-gray-500">
                      Automatisch Absturzberichte und Fehler melden
                    </p>
                  </div>
                  <IOSToggle
                    checked={activityTracking}
                    onChange={setActivityTracking}
                  />
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                Daten exportieren
              </h3>
              <p className="mb-4 text-sm text-gray-600">
                Laden Sie eine Kopie Ihrer Daten herunter.
              </p>
              <button className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100">
                Daten exportieren
              </button>
            </div>
          </div>
        );

      case "data":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                Datenbankzugriff
              </h3>
              <p className="mb-4 text-sm text-gray-600">
                Verwalten Sie die Datenbank und Systemeinstellungen.
              </p>
              <button
                onClick={() => router.push("/database")}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
              >
                Zur Datenbank
              </button>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                Backup & Wiederherstellung
              </h3>
              <p className="mb-4 text-sm text-gray-600">
                Erstellen Sie Backups und stellen Sie Daten wieder her.
              </p>
              <div className="flex gap-3">
                <button className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100">
                  Backup erstellen
                </button>
                <button className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100">
                  Wiederherstellen
                </button>
              </div>
            </div>
          </div>
        );

      case "subscription":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl bg-gradient-to-br from-gray-900 to-gray-700 p-8 text-white">
              <h3 className="mb-2 text-2xl font-bold">Premium Plan</h3>
              <p className="mb-6 text-gray-300">Aktiv seit 01.01.2024</p>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                <div>
                  <p className="text-sm text-gray-400">Nutzer</p>
                  <p className="text-2xl font-bold">150 / 200</p>
                </div>
                <div>
                  <p className="text-sm text-gray-400">Speicher</p>
                  <p className="text-2xl font-bold">45 / 100 GB</p>
                </div>
                <div>
                  <p className="text-sm text-gray-400">Nächste Zahlung</p>
                  <p className="text-2xl font-bold">01.01.2025</p>
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-3 text-base font-semibold text-gray-900">
                Plan ändern
              </h3>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                <div className="rounded-xl border border-gray-200 p-4">
                  <h4 className="mb-2 font-semibold">Basic</h4>
                  <p className="mb-2 text-2xl font-bold">
                    €49<span className="text-sm text-gray-500">/Monat</span>
                  </p>
                  <ul className="space-y-1 text-sm text-gray-600">
                    <li>• 50 Nutzer</li>
                    <li>• 10 GB Speicher</li>
                    <li>• Basis-Support</li>
                  </ul>
                </div>
                <div className="relative rounded-xl border-2 border-gray-900 p-4">
                  <span className="absolute -top-3 left-4 bg-white px-2 text-xs font-semibold">
                    AKTUELL
                  </span>
                  <h4 className="mb-2 font-semibold">Premium</h4>
                  <p className="mb-2 text-2xl font-bold">
                    €99<span className="text-sm text-gray-500">/Monat</span>
                  </p>
                  <ul className="space-y-1 text-sm text-gray-600">
                    <li>• 200 Nutzer</li>
                    <li>• 100 GB Speicher</li>
                    <li>• Priority-Support</li>
                  </ul>
                </div>
                <div className="rounded-xl border border-gray-200 p-4">
                  <h4 className="mb-2 font-semibold">Enterprise</h4>
                  <p className="mb-2 text-2xl font-bold">
                    €299<span className="text-sm text-gray-500">/Monat</span>
                  </p>
                  <ul className="space-y-1 text-sm text-gray-600">
                    <li>• Unbegrenzte Nutzer</li>
                    <li>• 1 TB Speicher</li>
                    <li>• 24/7 Support</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        );

      case "payment":
        return (
          <div className="space-y-6">
            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Zahlungsmethoden
              </h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between rounded-xl border border-gray-200 p-4">
                  <div className="flex items-center gap-4">
                    <div className="flex h-8 w-12 items-center justify-center rounded bg-gradient-to-r from-blue-600 to-blue-400 text-xs font-bold text-white">
                      VISA
                    </div>
                    <div>
                      <p className="font-medium">•••• •••• •••• 4242</p>
                      <p className="text-sm text-gray-500">Läuft ab 12/25</p>
                    </div>
                  </div>
                  <span className="rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-700">
                    Standard
                  </span>
                </div>
                <button className="w-full rounded-xl border-2 border-dashed border-gray-300 p-4 text-gray-600 transition-all hover:border-gray-400 hover:text-gray-700">
                  + Neue Zahlungsmethode hinzufügen
                </button>
              </div>
            </div>

            <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
              <h3 className="mb-4 text-base font-semibold text-gray-900">
                Zahlungsverlauf
              </h3>
              <div className="space-y-3">
                <div className="flex items-center justify-between border-b border-gray-100 py-3">
                  <div>
                    <p className="font-medium">Premium Plan - November 2024</p>
                    <p className="text-sm text-gray-500">01.11.2024</p>
                  </div>
                  <p className="font-semibold">€99.00</p>
                </div>
                <div className="flex items-center justify-between border-b border-gray-100 py-3">
                  <div>
                    <p className="font-medium">Premium Plan - Oktober 2024</p>
                    <p className="text-sm text-gray-500">01.10.2024</p>
                  </div>
                  <p className="font-semibold">€99.00</p>
                </div>
                <div className="flex items-center justify-between py-3">
                  <div>
                    <p className="font-medium">Premium Plan - September 2024</p>
                    <p className="text-sm text-gray-500">01.09.2024</p>
                  </div>
                  <p className="font-semibold">€99.00</p>
                </div>
              </div>
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
                const isDisabled = tab.disabled;
                return (
                  <button
                    key={tab.id}
                    onClick={() => !isDisabled && setActiveTab(tab.id)}
                    disabled={isDisabled}
                    className={`relative flex items-center gap-2 pb-3 text-sm font-medium transition-all ${
                      isDisabled
                        ? "cursor-not-allowed text-gray-300 opacity-50"
                        : isActive
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
                    {isDisabled && (
                      <span className="ml-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                        Bald
                      </span>
                    )}
                    {isActive && !isDisabled && (
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
            {/* Mobile List View - iOS Grouped Style */}
            {activeTab === null ? (
              <div className="flex flex-col space-y-6 pb-6">
                {/* General Settings Section */}
                <div className="space-y-2">
                  <h3 className="px-6 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                    Allgemein
                  </h3>
                  <div className="mx-4 overflow-hidden rounded-3xl bg-white shadow-sm">
                    {tabs
                      .filter((tab) => !tab.adminOnly)
                      .map((tab, index, arr) => {
                        const isDisabled = tab.disabled;
                        return (
                          <button
                            key={tab.id}
                            onClick={() =>
                              !isDisabled && handleTabSelect(tab.id)
                            }
                            disabled={isDisabled}
                            className={`flex w-full items-center justify-between px-4 py-4 transition-colors ${
                              isDisabled
                                ? "cursor-not-allowed opacity-40"
                                : "hover:bg-gray-50 active:bg-gray-100"
                            } ${
                              index !== arr.length - 1
                                ? "border-b border-gray-100"
                                : ""
                            }`}
                          >
                            <div className="flex items-center gap-4">
                              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-gray-100 to-gray-50">
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
                                    d={tab.icon}
                                  />
                                </svg>
                              </div>
                              <div className="flex items-center gap-2">
                                <p className="text-base font-medium text-gray-900">
                                  {tab.label}
                                </p>
                                {isDisabled && (
                                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                                    Bald
                                  </span>
                                )}
                              </div>
                            </div>
                            {!isDisabled && (
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
                            )}
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
