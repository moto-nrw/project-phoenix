"use client";

import { Suspense, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { PasswordChangeModal } from "~/components/ui";
import { PINManagement } from "~/components/staff";
import { SettingsCard, SimpleToggle, SimpleAlert } from "~/components/simple";

// Icons as simple SVG components
const SettingsIcon = () => (
  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
  </svg>
);

const NotificationIcon = () => (
  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
  </svg>
);

const SecurityIcon = () => (
  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
  </svg>
);

const DatabaseIcon = () => (
  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
  </svg>
);


function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const [activeSection, setActiveSection] = useState<string | null>("general");
  const [showAlert, setShowAlert] = useState(false);
  const [emailNotifications, setEmailNotifications] = useState(true);
  const [systemNotifications, setSystemNotifications] = useState(true);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  if (status === "loading") {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-[#5080D8]"></div>
      </div>
    );
  }

  if (!session?.user) {
    redirect("/");
  }

  const handleSave = () => {
    setShowAlert(true);
  };

  // Settings sections configuration
  const settingsSections = [
    {
      id: "general",
      title: "Allgemeine Einstellungen",
      description: "Profil und grundlegende Präferenzen",
      icon: <SettingsIcon />,
    },
    {
      id: "notifications",
      title: "Benachrichtigungen",
      description: "E-Mail und System-Benachrichtigungen",
      icon: <NotificationIcon />,
    },
    {
      id: "security",
      title: "Sicherheit",
      description: "Passwort und PIN verwalten",
      icon: <SecurityIcon />,
    },
    {
      id: "database",
      title: "Datenverwaltung",
      description: "Datenbank-Administration",
      icon: <DatabaseIcon />,
      href: "/database",
      badge: "Admin",
    },
  ];

  const renderSectionContent = () => {
    switch (activeSection) {
      case "general":
        return (
          <div className="space-y-6">
            {/* Profile Link Card */}
            <Link
              href="/profile"
              className="block p-6 bg-white rounded-2xl border border-gray-100 hover:border-gray-200 hover:shadow-md transition-all active:scale-[0.98]"
            >
              <div className="flex items-center gap-4">
                <div className="w-14 h-14 rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070C8] flex items-center justify-center text-white text-lg font-bold shadow-md">
                  {session?.user?.name?.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2) ?? 'U'}
                </div>
                <div className="flex-1">
                  <h4 className="font-semibold text-gray-900">Profil bearbeiten</h4>
                  <p className="text-sm text-gray-600 mt-1">Name und E-Mail ändern</p>
                </div>
                <svg className="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </Link>
          </div>
        );

      case "notifications":
        return (
          <div className="space-y-4">
            <SimpleToggle
              label="E-Mail-Benachrichtigungen"
              description="Wichtige Updates per E-Mail erhalten"
              checked={emailNotifications}
              onChange={setEmailNotifications}
            />
            <SimpleToggle
              label="System-Benachrichtigungen"
              description="In-App-Benachrichtigungen für Aktivitäten"
              checked={systemNotifications}
              onChange={setSystemNotifications}
            />
          </div>
        );

      case "security":
        return (
          <div className="space-y-6">
            {/* Password Change Button */}
            <button
              onClick={() => setShowPasswordModal(true)}
              className="w-full p-4 bg-white rounded-2xl border border-gray-100 hover:border-gray-200 hover:shadow-md transition-all text-left active:scale-[0.98]"
            >
              <div className="flex items-center justify-between">
                <div>
                  <h4 className="font-medium text-gray-900">Passwort ändern</h4>
                  <p className="text-sm text-gray-600 mt-1">Konto-Passwort aktualisieren</p>
                </div>
                <svg className="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </button>

            {/* PIN Management */}
            <div className="bg-white rounded-2xl border border-gray-100 p-6">
              <PINManagement onSuccess={handleSave} />
            </div>
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <>
      {/* Header */}
      <div className="mb-6 md:mb-8">
        <h1 className="text-3xl md:text-4xl font-bold text-gray-900">Einstellungen</h1>
        <p className="mt-2 text-gray-600">Persönliche Einstellungen verwalten</p>
      </div>

      {/* Mobile: Accordion Style */}
      <div className="lg:hidden space-y-3">
        {settingsSections.map((section, index) => (
          <SettingsCard
            key={section.id}
            title={section.title}
            description={section.description}
            icon={section.icon}
            href={section.href}
            onClick={section.href ? undefined : () => setActiveSection(activeSection === section.id ? null : section.id)}
            isActive={activeSection === section.id}
            badge={section.badge}
            index={index}
          >
            {activeSection === section.id && !section.href && (
              <div className="mt-6 pt-6 border-t border-gray-100">
                {renderSectionContent()}
                {(activeSection === "general" || activeSection === "notifications") && (
                  <div className="mt-6">
                    <button
                      onClick={handleSave}
                      className="w-full px-6 py-3 bg-[#83CD2D] text-white font-medium rounded-xl hover:bg-[#70B525] active:scale-95 transition-all"
                    >
                      Speichern
                    </button>
                  </div>
                )}
              </div>
            )}
          </SettingsCard>
        ))}
      </div>

      {/* Desktop: Side Navigation */}
      <div className="hidden lg:grid lg:grid-cols-3 lg:gap-6">
        <div className="space-y-3">
          {settingsSections.map((section, index) => (
            <SettingsCard
              key={section.id}
              title={section.title}
              description={section.description}
              icon={section.icon}
              href={section.href}
              onClick={section.href ? undefined : () => setActiveSection(section.id)}
              isActive={activeSection === section.id}
              badge={section.badge}
              index={index}
            />
          ))}
        </div>

        <div className="lg:col-span-2">
          <div className="bg-white rounded-3xl shadow-md p-8">
            <h2 className="text-xl font-semibold text-gray-900 mb-6">
              {settingsSections.find(s => s.id === activeSection)?.title}
            </h2>
            {renderSectionContent()}
            {(activeSection === "general" || activeSection === "notifications") && (
              <div className="mt-8 pt-6 border-t border-gray-100">
                <div className="flex justify-end">
                  <button
                    onClick={handleSave}
                    className="px-8 py-3 bg-[#83CD2D] text-white font-medium rounded-xl hover:bg-[#70B525] active:scale-95 transition-all"
                  >
                    Änderungen speichern
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Success Alert */}
      {showAlert && (
        <SimpleAlert
          type="success"
          message="Einstellungen erfolgreich gespeichert"
          autoClose
          onClose={() => setShowAlert(false)}
        />
      )}

      {/* Password Change Modal */}
      <PasswordChangeModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        onSuccess={handleSave}
      />

      {/* Add floating animation styles */}
      <style jsx global>{`
        @keyframes float {
          0%, 100% {
            transform: translateY(0px) rotate(var(--rotation));
          }
          50% {
            transform: translateY(-4px) rotate(var(--rotation));
          }
        }
      `}</style>
    </>
  );
}

export default function SettingsPage() {
  return (
    <ResponsiveLayout>
      <Suspense
        fallback={
          <div className="flex min-h-[50vh] items-center justify-center">
            <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-[#5080D8]"></div>
          </div>
        }
      >
        <SettingsContent />
      </Suspense>
    </ResponsiveLayout>
  );
}