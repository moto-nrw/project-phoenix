"use client";

import { Suspense, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { Button, PasswordChangeModal } from "~/components/ui";

// Simple icon component
const Icon: React.FC<{ path: string; className?: string }> = ({ path, className }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

interface SettingsSection {
  id: string;
  title: string;
  description: string;
  iconPath: string;
  href?: string;
  action?: () => void;
  badge?: string;
  warning?: boolean;
}

const settingsSections: SettingsSection[] = [
  {
    id: "general",
    title: "Allgemeine Einstellungen",
    description: "Sprache, Design und grundlegende Präferenzen",
    iconPath: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
    href: "#general",
  },
  {
    id: "notifications",
    title: "Benachrichtigungen",
    description: "E-Mail und System-Benachrichtigungen verwalten",
    iconPath: "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9",
    href: "#notifications",
  },
  {
    id: "security",
    title: "Sicherheit & Datenschutz",
    description: "Passwort ändern und Datenschutzeinstellungen",
    iconPath: "M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z",
    href: "#security",
  },
  {
    id: "database",
    title: "Datenverwaltung",
    description: "Schüler, Lehrer, Räume und Aktivitäten verwalten",
    iconPath: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4",
    href: "/database",
    badge: "Admin",
  },
];


function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const [activeSection, setActiveSection] = useState<string | null>("general");
  const [showSaveAlert, setShowSaveAlert] = useState(false);
  const [emailNotifications, setEmailNotifications] = useState(true);
  const [systemNotifications, setSystemNotifications] = useState(true);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  if (status === "loading") {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
      </div>
    );
  }

  if (!session?.user) {
    redirect("/");
  }


  const handleSave = () => {
    // Here you would normally save the settings
    setShowSaveAlert(true);
    setTimeout(() => setShowSaveAlert(false), 3000);
  };

  const SettingCard: React.FC<{
    section: SettingsSection;
    onClick?: () => void;
    isActive?: boolean;
  }> = ({ section, onClick, isActive }) => {
    const isExternalLink = section.href?.startsWith('/');
    
    if (isExternalLink && section.href) {
      return (
        <Link
          href={section.href}
          className={`block w-full rounded-lg border p-4 text-left transition-all duration-200 hover:shadow-md ${
            isActive
              ? "border-gray-900 bg-gray-50 shadow-md"
              : "border-gray-200 bg-white hover:border-gray-300"
          }`}
        >
          <div className="flex items-start justify-between">
            <div className="flex items-start space-x-3">
              <div className={`rounded-lg p-2 ${isActive ? "bg-gray-900" : "bg-gray-100"}`}>
                <Icon path={section.iconPath} className={`h-5 w-5 ${isActive ? "text-white" : "text-gray-600"}`} />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-gray-900">{section.title}</h3>
                  {section.badge && (
                    <span className="rounded-full bg-gray-900 px-2 py-0.5 text-xs font-medium text-white">
                      {section.badge}
                    </span>
                  )}
                </div>
                <p className="mt-1 text-sm text-gray-600">{section.description}</p>
              </div>
            </div>
            <Icon path="M8.25 4.5l7.5 7.5-7.5 7.5" className="h-5 w-5 text-gray-400" />
          </div>
        </Link>
      );
    }
    
    return (
      <button
        onClick={onClick ?? (() => setActiveSection(section.id))}
        className={`w-full rounded-lg border p-4 text-left transition-all duration-200 hover:shadow-md ${
          isActive
            ? "border-gray-900 bg-gray-50 shadow-md"
            : "border-gray-200 bg-white hover:border-gray-300"
        }`}
      >
        <div className="flex items-start justify-between">
          <div className="flex items-start space-x-3">
            <div className={`rounded-lg p-2 ${isActive ? "bg-gray-900" : "bg-gray-100"}`}>
              <Icon path={section.iconPath} className={`h-5 w-5 ${isActive ? "text-white" : "text-gray-600"}`} />
            </div>
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-gray-900">{section.title}</h3>
                {section.badge && (
                  <span className="rounded-full bg-gray-900 px-2 py-0.5 text-xs font-medium text-white">
                    {section.badge}
                  </span>
                )}
              </div>
              <p className="mt-1 text-sm text-gray-600">{section.description}</p>
            </div>
          </div>
          <Icon path="M8.25 4.5l7.5 7.5-7.5 7.5" className="h-5 w-5 text-gray-400" />
        </div>
      </button>
    );
  };

  const renderSectionContent = () => {
    switch (activeSection) {
      case "general":
        return (
          <div className="space-y-6">
            <div>
              <h3 className="text-lg font-semibold text-gray-900">Allgemeine Einstellungen</h3>
              <p className="mt-1 text-sm text-gray-600">
                Passen Sie die grundlegenden Einstellungen der Anwendung an
              </p>
            </div>

            <div className="space-y-4">
              {/* Profile Management Link */}
              <Link
                href="/profile"
                className="block rounded-lg border border-gray-200 p-4 transition-all hover:border-gray-300 hover:shadow-md group"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <div className="w-12 h-12 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white shadow-md group-hover:shadow-lg transition-all">
                      <span className="text-lg font-bold">
                        {session?.user?.name ? 
                          session.user.name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2) : 
                          'U'
                        }
                      </span>
                    </div>
                    <div>
                      <h4 className="font-medium text-gray-900">Profil verwalten</h4>
                      <p className="text-sm text-gray-600">Name, E-Mail und Passwort ändern</p>
                    </div>
                  </div>
                  <Icon path="M8.25 4.5l7.5 7.5-7.5 7.5" className="h-5 w-5 text-gray-400 group-hover:translate-x-1 transition-transform" />
                </div>
              </Link>


              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Design</label>
                <div className="grid grid-cols-2 gap-4">
                  <button
                    className="rounded-lg border-2 p-4 text-center transition-colors border-gray-900 bg-gray-50"
                  >
                    <Icon path="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" className="mx-auto h-6 w-6 text-[#F78C10]" />
                    <span className="mt-2 block text-sm font-medium text-gray-900">Hell</span>
                  </button>
                  <button
                    className="rounded-lg border-2 p-4 text-center transition-colors border-gray-200 hover:border-gray-300"
                  >
                    <Icon path="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" className="mx-auto h-6 w-6 text-gray-600" />
                    <span className="mt-2 block text-sm font-medium text-gray-900">Dunkel</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        );

      case "notifications":
        return (
          <div className="space-y-6">
            <div>
              <h3 className="text-lg font-semibold text-gray-900">Benachrichtigungen</h3>
              <p className="mt-1 text-sm text-gray-600">
                Verwalten Sie, wie und wann Sie Benachrichtigungen erhalten
              </p>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between rounded-lg border border-gray-200 p-4">
                <div>
                  <h4 className="font-medium text-gray-900">E-Mail-Benachrichtigungen</h4>
                  <p className="text-sm text-gray-600">
                    Erhalten Sie wichtige Updates per E-Mail
                  </p>
                </div>
                <button
                  onClick={() => setEmailNotifications(!emailNotifications)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    emailNotifications ? "bg-[#83CD2D]" : "bg-gray-200"
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform shadow-sm ${
                      emailNotifications ? "translate-x-6" : "translate-x-1"
                    }`}
                  />
                </button>
              </div>

              <div className="flex items-center justify-between rounded-lg border border-gray-200 p-4">
                <div>
                  <h4 className="font-medium text-gray-900">System-Benachrichtigungen</h4>
                  <p className="text-sm text-gray-600">
                    In-App-Benachrichtigungen für Aktivitäten
                  </p>
                </div>
                <button
                  onClick={() => setSystemNotifications(!systemNotifications)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    systemNotifications ? "bg-[#83CD2D]" : "bg-gray-200"
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform shadow-sm ${
                      systemNotifications ? "translate-x-6" : "translate-x-1"
                    }`}
                  />
                </button>
              </div>
            </div>
          </div>
        );

      case "security":
        return (
          <div className="space-y-6">
            <div>
              <h3 className="text-lg font-semibold text-gray-900">Sicherheit & Datenschutz</h3>
              <p className="mt-1 text-sm text-gray-600">
                Schützen Sie Ihr Konto und verwalten Sie Ihre Datenschutzeinstellungen
              </p>
            </div>

            <div className="space-y-4">
              <button
                onClick={() => setShowPasswordModal(true)}
                className="w-full flex items-center justify-between rounded-lg border border-gray-200 p-4 transition-colors hover:bg-gray-50"
              >
                <div className="flex items-center space-x-3">
                  <Icon path="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" className="h-5 w-5 text-gray-600" />
                  <div className="text-left">
                    <h4 className="font-medium text-gray-900">Passwort ändern</h4>
                    <p className="text-sm text-gray-600">
                      Aktualisieren Sie Ihr Konto-Passwort
                    </p>
                  </div>
                </div>
                <Icon path="M8.25 4.5l7.5 7.5-7.5 7.5" className="h-5 w-5 text-gray-400" />
              </button>

              <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-4">
                <div className="flex items-start space-x-3">
                  <Icon path="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" className="h-5 w-5 text-yellow-600" />
                  <div>
                    <h4 className="font-medium text-gray-900">Datenschutz-Hinweis</h4>
                    <p className="mt-1 text-sm text-gray-600">
                      Diese Anwendung verarbeitet personenbezogene Daten gemäß DSGVO. Weitere
                      Informationen finden Sie in unserer Datenschutzerklärung.
                    </p>
                  </div>
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
    <>
      {/* Header */}
      <div className="mb-6 md:mb-8">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h1 className="text-2xl md:text-3xl font-bold text-gray-900">Einstellungen</h1>
            <p className="mt-1 text-sm md:text-base text-gray-600">
              Verwalten Sie Ihre persönlichen Einstellungen und Systemkonfigurationen
            </p>
          </div>
        </div>
      </div>

      {/* Mobile: Full page navigation */}
      <div className="block lg:hidden">
        {activeSection === null ? (
          <div className="space-y-3">
            {settingsSections.map((section) => (
              <SettingCard
                key={section.id}
                section={section}
                onClick={() => setActiveSection(section.id)}
              />
            ))}
          </div>
        ) : (
          <div>
            <button
              onClick={() => setActiveSection(null)}
              className="mb-4 flex items-center text-sm text-gray-600 hover:text-gray-900"
            >
              <Icon path="M8.25 4.5l7.5 7.5-7.5 7.5" className="mr-1 h-4 w-4 rotate-180" />
              Zurück zu Einstellungen
            </button>
            <div className="rounded-lg border border-gray-200 bg-white p-4 md:p-6">
              {renderSectionContent()}
            </div>
          </div>
        )}
      </div>

      {/* Desktop: Side navigation with content */}
      <div className="hidden lg:grid lg:grid-cols-3 lg:gap-6">
        <div className="space-y-3">
          {settingsSections.map((section) => (
            <SettingCard
              key={section.id}
              section={section}
              isActive={activeSection === section.id}
            />
          ))}
        </div>

        <div className="lg:col-span-2">
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            {renderSectionContent()}

            {/* Save button for applicable sections */}
            {(activeSection === "general" || activeSection === "notifications") && (
              <div className="mt-6 border-t border-gray-100 pt-6">
                <div className="flex justify-end">
                  <Button 
                    onClick={handleSave} 
                    variant="success"
                    className="relative min-w-[200px] overflow-hidden"
                  >
                    {/* Gradient overlay that moves on hover */}
                    <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent -translate-x-full group-hover:translate-x-full transition-transform duration-1000 ease-out" />
                    
                    {/* Button content */}
                    <span className="relative flex items-center gap-2.5">
                      <span className="relative flex items-center justify-center">
                        {/* Animated ring around icon */}
                        <span className="absolute h-7 w-7 rounded-full bg-white/20 scale-0 group-hover:scale-110 transition-transform duration-300 ease-out" />
                        <Icon 
                          path="M5 13l4 4L19 7" 
                          className="relative h-5 w-5 transition-transform duration-200"
                        />
                      </span>
                      <span className="font-medium tracking-wide">Änderungen speichern</span>
                    </span>
                    
                    {/* Subtle pulse on hover */}
                    <span className="absolute inset-0 rounded-lg ring-2 ring-[#83CD2D] ring-opacity-0 group-hover:ring-opacity-30 transition-all duration-300 group-hover:scale-105 pointer-events-none" />
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Save confirmation alert */}
      {showSaveAlert && (
        <div className="fixed bottom-6 right-6 z-50 animate-slideInRight">
          <div className="relative">
            {/* Backdrop blur effect */}
            <div className="absolute inset-0 bg-white/80 backdrop-blur-xl rounded-2xl" />
            
            {/* Content */}
            <div className="relative flex items-center gap-3 rounded-2xl border border-[#83CD2D]/30 bg-gradient-to-br from-[#83CD2D]/10 to-[#83CD2D]/5 px-5 py-4 shadow-2xl shadow-[#83CD2D]/10">
              {/* Success icon with animation */}
              <div className="relative">
                <div className="absolute inset-0 bg-[#83CD2D] rounded-full blur-xl opacity-25 animate-pulse" />
                <div className="relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525]">
                  <Icon 
                    path="M5 13l4 4L19 7" 
                    className="h-5 w-5 text-white"
                  />
                </div>
              </div>
              
              {/* Text */}
              <div>
                <p className="font-semibold text-gray-900">Erfolgreich gespeichert</p>
                <p className="text-sm text-gray-600">Ihre Einstellungen wurden übernommen</p>
              </div>
              
              {/* Progress bar */}
              <div className="absolute bottom-0 left-0 right-0 h-1 overflow-hidden rounded-b-2xl">
                <div className="h-full bg-gradient-to-r from-[#83CD2D] to-[#70b525] animate-[shrink_3s_linear_forwards]" />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Password Change Modal */}
      <PasswordChangeModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        onSuccess={() => {
          setShowSaveAlert(true);
          setTimeout(() => setShowSaveAlert(false), 3000);
        }}
      />
    </>
  );
}

export default function SettingsPage() {
  return (
    <ResponsiveLayout>
      <Suspense
        fallback={
          <div className="flex min-h-[50vh] items-center justify-center">
            <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          </div>
        }
      >
        <SettingsContent />
      </Suspense>
    </ResponsiveLayout>
  );
}