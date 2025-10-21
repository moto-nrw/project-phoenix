"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { PasswordChangeModal } from "~/components/ui";
import { PINManagement } from "~/components/staff";
import { IOSToggle } from "~/components/ui/ios-toggle";
import { fetchProfile, updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { Profile, ProfileUpdateRequest } from "~/lib/profile-helpers";

// Tab configuration
interface Tab {
  id: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
}

const tabs: Tab[] = [
  { id: "profile", label: "Profil", icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" },
  { id: "security", label: "Sicherheit", icon: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" },
  { id: "notifications", label: "Benachrichtigungen", icon: "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" },
  { id: "appearance", label: "Darstellung", icon: "M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" },
  { id: "privacy", label: "Privatsphäre", icon: "M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" },
];

const adminTabs: Tab[] = [
  { id: "data", label: "Datenverwaltung", icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4", adminOnly: true },
  { id: "subscription", label: "Abonnement", icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01", adminOnly: true },
  { id: "payment", label: "Zahlungsmethode", icon: "M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z", adminOnly: true },
];

function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<string | null>("profile");
  const [showAlert, setShowAlert] = useState(false);
  const [alertMessage, setAlertMessage] = useState("");
  const [alertType, setAlertType] = useState<"success" | "error">("success");
  const [isMobile, setIsMobile] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  // Profile state
  const [profile, setProfile] = useState<Profile | null>(null);
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

  useEffect(() => {
    if (session?.user?.token) {
      void loadProfile();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session]);

  const loadProfile = async () => {
    if (!session?.user?.token) return;
    
    try {
      const profileData = await fetchProfile();
      setProfile(profileData);
      setFormData({
        firstName: profileData.firstName || "",
        lastName: profileData.lastName || "",
        email: profileData.email || "",
      });
    } catch (error) {
      console.error("Failed to load profile:", error);
    }
  };

  const handleSaveProfile = async () => {
    if (!session?.user?.token || !profile) return;
    
    setIsSaving(true);
    try {
      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
      };
      
      await updateProfile(updateData);
      await loadProfile();
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

  const handleAvatarChange = async (file: File) => {
    if (!session?.user?.token) return;
    
    try {
      await uploadAvatar(file);
      await loadProfile();
      setAlertMessage("Profilbild erfolgreich aktualisiert");
      setAlertType("success");
      setShowAlert(true);
    } catch {
      setAlertMessage("Fehler beim Hochladen des Profilbilds");
      setAlertType("error");
      setShowAlert(true);
    }
  };

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-gray-800"></div>
          <p className="text-gray-600">Einstellungen werden geladen...</p>
        </div>
      </div>
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
            <div className={`mb-8 ${isMobile ? 'flex justify-center' : ''}`}>
              <div className="inline-block">
                <div className="flex flex-col items-center">
                  <div className="relative group">
                    <div className="w-32 h-32 rounded-full overflow-hidden bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white shadow-xl">
                      {profile?.avatar ? (
                        // eslint-disable-next-line @next/next/no-img-element
                        <img src={profile.avatar} alt="Profile" className="w-full h-full object-cover" />
                      ) : (
                        <span className="text-4xl font-bold">
                          {(formData.firstName?.charAt(0) || "") + (formData.lastName?.charAt(0) || "")}
                        </span>
                      )}
                    </div>
                    <label htmlFor="avatar-upload" className="absolute inset-0 flex items-center justify-center bg-black/50 rounded-full opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
                      <svg className="w-8 h-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
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
                    onClick={() => document.getElementById('avatar-upload')?.click()}
                    className="mt-3 text-[11px] text-gray-600 hover:text-gray-900 font-medium transition-colors"
                  >
                    Profilbild ändern
                  </button>
                </div>
              </div>
            </div>

            {/* Profile Form - Better mobile layout */}
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-4 md:p-6 border border-gray-100">
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Vorname</label>
                  <input
                    type="text"
                    value={formData.firstName}
                    onChange={(e) => setFormData({ ...formData, firstName: e.target.value })}
                    disabled={!isEditing}
                    className="w-full px-4 py-3 rounded-lg border border-gray-200 text-base focus:outline-none focus:ring-2 focus:ring-[#5080D8] disabled:bg-gray-50 disabled:text-gray-500 transition-all"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Nachname</label>
                  <input
                    type="text"
                    value={formData.lastName}
                    onChange={(e) => setFormData({ ...formData, lastName: e.target.value })}
                    disabled={!isEditing}
                    className="w-full px-4 py-3 rounded-lg border border-gray-200 text-base focus:outline-none focus:ring-2 focus:ring-[#5080D8] disabled:bg-gray-50 disabled:text-gray-500 transition-all"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">E-Mail</label>
                  <input
                    type="email"
                    value={formData.email}
                    disabled
                    className="w-full px-4 py-3 rounded-lg border border-gray-200 text-base bg-gray-50 text-gray-500"
                  />
                </div>
              </div>
            </div>

            {/* Action Buttons */}
            <div className={`flex gap-3 ${isMobile ? 'justify-center' : ''}`}>
              {isEditing ? (
                <>
                  <button
                    onClick={() => {
                      setIsEditing(false);
                      void loadProfile();
                    }}
                    className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200"
                  >
                    Abbrechen
                  </button>
                  <button
                    onClick={() => void handleSaveProfile()}
                    disabled={isSaving}
                    className="px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 disabled:opacity-50 disabled:hover:scale-100 transition-all duration-200"
                  >
                    {isSaving ? "Speichern..." : "Speichern"}
                  </button>
                </>
              ) : (
                <button
                  onClick={() => setIsEditing(true)}
                  className="px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
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
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Passwort ändern</h3>
              <p className="text-sm text-gray-600 mb-4">Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit.</p>
              <button
                onClick={() => setShowPasswordModal(true)}
                className="px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
              >
                Passwort ändern
              </button>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">PIN-Verwaltung</h3>
              <p className="text-sm text-gray-600 mb-4">Verwalten Sie Ihre PIN für RFID-Geräte.</p>
              <PINManagement
                onSuccess={(message) => {
                  setAlertMessage(message);
                  setAlertType("success");
                  setShowAlert(true);
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
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Kritische Benachrichtigungen</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">Notfallmeldungen</p>
                    <p className="text-xs text-gray-500">Sofortige Benachrichtigung bei Notfällen</p>
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
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">Nicht abgeholte Kinder</p>
                    <p className="text-xs text-gray-500">Erinnerung wenn Kinder nach Schließzeit noch da sind</p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={emailNotifications}
                      onChange={setEmailNotifications}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">Vertretungsanfragen</p>
                    <p className="text-xs text-gray-500">Benachrichtigung bei Vertretungsbedarf</p>
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

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Wöchentlicher Statusbericht</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">OGS-Betriebsübersicht</p>
                    <p className="text-xs text-gray-500">Zusammenfassung: Anwesenheit, Aktivitäten, Vorfälle</p>
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

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Benachrichtigungskanal</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">E-Mail-Benachrichtigungen</p>
                    <p className="text-xs text-gray-500">Benachrichtigungen per E-Mail erhalten</p>
                  </div>
                  <div className="flex-shrink-0">
                    <IOSToggle
                      checked={emailChannel}
                      onChange={setEmailChannel}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900">Browser-Benachrichtigungen</p>
                    <p className="text-xs text-gray-500">Push-Benachrichtigungen im Browser</p>
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
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Design</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">Dunkler Modus</p>
                    <p className="text-xs text-gray-500">Augenschonendes Design für dunklere Umgebungen</p>
                  </div>
                  <IOSToggle
                    checked={darkMode}
                    onChange={setDarkMode}
                  />
                </div>
              </div>
            </div>
          </div>
        );

      case "privacy":
        return (
          <div className="space-y-6">
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Datenschutz</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">Daten mit moto teilen</p>
                    <p className="text-xs text-gray-500">Ihre Daten werden anonymisiert geteilt</p>
                  </div>
                  <IOSToggle
                    checked={dataSharing}
                    onChange={setDataSharing}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">Fehlerberichte senden</p>
                    <p className="text-xs text-gray-500">Automatisch Absturzberichte und Fehler melden</p>
                  </div>
                  <IOSToggle
                    checked={activityTracking}
                    onChange={setActivityTracking}
                  />
                </div>
              </div>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Daten exportieren</h3>
              <p className="text-sm text-gray-600 mb-4">Laden Sie eine Kopie Ihrer Daten herunter.</p>
              <button className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200">
                Daten exportieren
              </button>
            </div>
          </div>
        );

      case "data":
        return (
          <div className="space-y-6">
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Datenbankzugriff</h3>
              <p className="text-sm text-gray-600 mb-4">Verwalten Sie die Datenbank und Systemeinstellungen.</p>
              <button
                onClick={() => router.push("/database")}
                className="px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
              >
                Zur Datenbank
              </button>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Backup & Wiederherstellung</h3>
              <p className="text-sm text-gray-600 mb-4">Erstellen Sie Backups und stellen Sie Daten wieder her.</p>
              <div className="flex gap-3">
                <button className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200">
                  Backup erstellen
                </button>
                <button className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200">
                  Wiederherstellen
                </button>
              </div>
            </div>
          </div>
        );

      case "subscription":
        return (
          <div className="space-y-6">
            <div className="bg-gradient-to-br from-gray-900 to-gray-700 rounded-2xl p-8 text-white">
              <h3 className="text-2xl font-bold mb-2">Premium Plan</h3>
              <p className="text-gray-300 mb-6">Aktiv seit 01.01.2024</p>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <p className="text-gray-400 text-sm">Nutzer</p>
                  <p className="text-2xl font-bold">150 / 200</p>
                </div>
                <div>
                  <p className="text-gray-400 text-sm">Speicher</p>
                  <p className="text-2xl font-bold">45 / 100 GB</p>
                </div>
                <div>
                  <p className="text-gray-400 text-sm">Nächste Zahlung</p>
                  <p className="text-2xl font-bold">01.01.2025</p>
                </div>
              </div>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Plan ändern</h3>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="border border-gray-200 rounded-xl p-4">
                  <h4 className="font-semibold mb-2">Basic</h4>
                  <p className="text-2xl font-bold mb-2">€49<span className="text-sm text-gray-500">/Monat</span></p>
                  <ul className="text-sm text-gray-600 space-y-1">
                    <li>• 50 Nutzer</li>
                    <li>• 10 GB Speicher</li>
                    <li>• Basis-Support</li>
                  </ul>
                </div>
                <div className="border-2 border-gray-900 rounded-xl p-4 relative">
                  <span className="absolute -top-3 left-4 bg-white px-2 text-xs font-semibold">AKTUELL</span>
                  <h4 className="font-semibold mb-2">Premium</h4>
                  <p className="text-2xl font-bold mb-2">€99<span className="text-sm text-gray-500">/Monat</span></p>
                  <ul className="text-sm text-gray-600 space-y-1">
                    <li>• 200 Nutzer</li>
                    <li>• 100 GB Speicher</li>
                    <li>• Priority-Support</li>
                  </ul>
                </div>
                <div className="border border-gray-200 rounded-xl p-4">
                  <h4 className="font-semibold mb-2">Enterprise</h4>
                  <p className="text-2xl font-bold mb-2">€299<span className="text-sm text-gray-500">/Monat</span></p>
                  <ul className="text-sm text-gray-600 space-y-1">
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
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Zahlungsmethoden</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between p-4 border border-gray-200 rounded-xl">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-8 bg-gradient-to-r from-blue-600 to-blue-400 rounded flex items-center justify-center text-white text-xs font-bold">
                      VISA
                    </div>
                    <div>
                      <p className="font-medium">•••• •••• •••• 4242</p>
                      <p className="text-sm text-gray-500">Läuft ab 12/25</p>
                    </div>
                  </div>
                  <span className="px-3 py-1 bg-green-100 text-green-700 text-xs font-medium rounded-full">Standard</span>
                </div>
                <button className="w-full p-4 border-2 border-dashed border-gray-300 rounded-xl text-gray-600 hover:border-gray-400 hover:text-gray-700 transition-all">
                  + Neue Zahlungsmethode hinzufügen
                </button>
              </div>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-4">Zahlungsverlauf</h3>
              <div className="space-y-3">
                <div className="flex items-center justify-between py-3 border-b border-gray-100">
                  <div>
                    <p className="font-medium">Premium Plan - November 2024</p>
                    <p className="text-sm text-gray-500">01.11.2024</p>
                  </div>
                  <p className="font-semibold">€99.00</p>
                </div>
                <div className="flex items-center justify-between py-3 border-b border-gray-100">
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
      <div className="w-full -mt-1.5">
        {/* Header - Only show on mobile list view */}
        {isMobile && activeTab === null && (
          <PageHeaderWithSearch
            title="Einstellungen"
          />
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
                    className={`
                      relative pb-3 text-sm font-medium transition-all flex items-center gap-2
                      ${isActive
                        ? "text-gray-900 font-semibold"
                        : "text-gray-500 hover:text-gray-700"
                      }
                    `}
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={tab.icon} />
                    </svg>
                    <span>{tab.label}</span>
                    {tab.adminOnly && (
                      <span className="ml-1 px-1.5 py-0.5 bg-gray-200 rounded text-xs">Admin</span>
                    )}
                    {isActive && (
                      <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-gray-900 rounded-full" />
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
                  <h3 className="px-6 text-xs font-semibold text-gray-500 uppercase tracking-wider">Allgemein</h3>
                  <div className="bg-white rounded-3xl shadow-sm overflow-hidden mx-4">
                    {tabs.filter(tab => !tab.adminOnly).map((tab, index, arr) => (
                      <button
                        key={tab.id}
                        onClick={() => handleTabSelect(tab.id)}
                        className={`w-full flex items-center justify-between px-4 py-4 hover:bg-gray-50 active:bg-gray-100 transition-colors ${
                          index !== arr.length - 1 ? "border-b border-gray-100" : ""
                        }`}
                      >
                        <div className="flex items-center gap-4">
                          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-gray-100 to-gray-50 flex items-center justify-center">
                            <svg className="w-5 h-5 text-gray-700" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={tab.icon} />
                            </svg>
                          </div>
                          <p className="text-base font-medium text-gray-900">{tab.label}</p>
                        </div>
                        <svg className="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                        </svg>
                      </button>
                    ))}
                  </div>
                </div>

              </div>
            ) : (
              /* Mobile Detail View */
              <div className="flex flex-col h-[calc(100vh-120px)]">
                {/* Mobile Header with Back Button */}
                <div className="flex items-center gap-3 pb-4 mb-4">
                  <button
                    onClick={handleBackToList}
                    className="p-2 -ml-2 rounded-lg hover:bg-gray-100 active:bg-gray-200 transition-all"
                    aria-label="Zurück"
                  >
                    <svg className="w-5 h-5 text-gray-700" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                    </svg>
                  </button>
                  <h2 className="text-lg font-semibold text-gray-900">
                    {allTabs.find(t => t.id === activeTab)?.label}
                  </h2>
                </div>
                {/* Scrollable Content */}
                <div className="flex-1 overflow-y-auto -mx-4 px-4">
                  {renderTabContent()}
                </div>
              </div>
            )}
          </>
        )}

        {/* Desktop Content */}
        {!isMobile && (
          <div className="min-h-[60vh]">
            {renderTabContent()}
          </div>
        )}

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
            onSuccess={() => {
              setShowPasswordModal(false);
              setAlertMessage("Passwort erfolgreich geändert");
              setAlertType("success");
              setShowAlert(true);
            }}
          />
        )}
      </div>
    </ResponsiveLayout>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-gray-800"></div>
          <p className="text-gray-600">Einstellungen werden geladen...</p>
        </div>
      </div>
    }>
      <SettingsContent />
    </Suspense>
  );
}