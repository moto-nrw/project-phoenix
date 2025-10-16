"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Suspense, useState, useEffect, useCallback } from "react";

// Icon component
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

// Tab configuration
interface Tab {
  id: string;
  label: string;
  icon: string;
}

const tabs: Tab[] = [
  { id: "data", label: "Datenbestand", icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" },
  { id: "import", label: "Import", icon: "M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" },
];

// Base data sections configuration
const baseDataSections = [
  {
    id: "students",
    title: "Schüler",
    description: "Schülerdaten verwalten und bearbeiten",
    href: "/database/students",
    icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    color: "from-[#5080D8] to-[#4070c8]",
  },
  {
    id: "teachers",
    title: "Betreuer",
    description: "Daten der Betreuer und Zuordnungen verwalten",
    href: "/database/teachers",
    icon: "M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222",
    color: "from-[#F78C10] to-[#e57a00]",
  },
  {
    id: "rooms",
    title: "Räume",
    description: "Räume und Ausstattung verwalten",
    href: "/database/rooms",
    icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    color: "from-indigo-500 to-indigo-600",
  },
  {
    id: "activities",
    title: "Aktivitäten",
    description: "Aktivitäten und Zeitpläne verwalten",
    href: "/database/activities",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    color: "from-[#FF3130] to-[#e02020]",
  },
  {
    id: "groups",
    title: "Gruppen",
    description: "Gruppen und Kombinationen verwalten",
    href: "/database/groups",
    icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
    color: "from-[#83CD2D] to-[#70b525]",
  },
  {
    id: "roles",
    title: "Rollen",
    description: "Benutzerrollen und Berechtigungen verwalten",
    href: "/database/roles",
    icon: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
    color: "from-purple-500 to-purple-600",
  },
  {
    id: "devices",
    title: "Geräte",
    description: "IoT-Geräte und RFID-Reader verwalten",
    href: "/database/devices",
    icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
    color: "from-amber-500 to-yellow-600",
  },
  {
    id: "permissions",
    title: "Berechtigungen",
    description: "Systemberechtigungen ansehen",
    href: "/database/permissions",
    icon: "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1 1 21 9z",
    color: "from-pink-500 to-rose-500",
  },
];

function DatabaseContent() {
  const { data: session, status } = useSession({ required: true });
  const [activeTab, setActiveTab] = useState<string | null>("data");
  const [isMobile, setIsMobile] = useState(false);
  const [counts, setCounts] = useState<{
    students: number;
    teachers: number;
    rooms: number;
    activities: number;
    groups: number;
    roles: number;
    devices: number;
    permissionCount: number;
  }>({
    students: 0,
    teachers: 0,
    rooms: 0,
    activities: 0,
    groups: 0,
    roles: 0,
    devices: 0,
    permissionCount: 0,
  });
  const [permissions, setPermissions] = useState<{
    canViewStudents: boolean;
    canViewTeachers: boolean;
    canViewRooms: boolean;
    canViewActivities: boolean;
    canViewGroups: boolean;
    canViewRoles: boolean;
    canViewDevices: boolean;
    canViewPermissions: boolean;
  }>({
    canViewStudents: false,
    canViewTeachers: false,
    canViewRooms: false,
    canViewActivities: false,
    canViewGroups: false,
    canViewRoles: false,
    canViewDevices: false,
    canViewPermissions: false,
  });
  const [countsLoading, setCountsLoading] = useState(true);

  // (removed unused local handlers to satisfy lint)

  // Mobile detection
  useEffect(() => {
    const handleResize = () => {
      const isNowMobile = window.innerWidth < 768;
      setIsMobile(isNowMobile);

      // Always show data tab
      if (activeTab === null) {
        setActiveTab("data");
      }
    };
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [activeTab]);

  // Fetch real counts from the database via Next.js API route
  useEffect(() => {
    const fetchCounts = async () => {
      try {
        const response = await fetch("/api/database/counts");
        if (response.ok) {
          const result = await response.json() as {
            success: boolean;
            message: string;
            data: {
              students: number;
              teachers: number;
              rooms: number;
              activities: number;
              groups: number;
              roles: number;
              devices: number;
              permissionCount: number;
              permissions: {
                canViewStudents: boolean;
                canViewTeachers: boolean;
                canViewRooms: boolean;
                canViewActivities: boolean;
                canViewGroups: boolean;
                canViewRoles: boolean;
                canViewDevices: boolean;
                canViewPermissions: boolean;
              };
            };
          };
          const data = result.data;
          setCounts({
            students: data.students,
            teachers: data.teachers,
            rooms: data.rooms,
            activities: data.activities,
            groups: data.groups,
            roles: data.roles,
            devices: data.devices,
            permissionCount: data.permissionCount,
          });
          setPermissions(data.permissions || {
            canViewStudents: false,
            canViewTeachers: false,
            canViewRooms: false,
            canViewActivities: false,
            canViewGroups: false,
            canViewRoles: false,
            canViewDevices: false,
            canViewPermissions: false,
          });
        } else {
          // Gracefully handle unauthorized/forbidden without noisy logs
          if (response.status === 401 || response.status === 403) {
            // Keep zeros and disable all sections via permissions
            setCounts({
              students: 0,
              teachers: 0,
              rooms: 0,
              activities: 0,
              groups: 0,
              roles: 0,
              devices: 0,
              permissionCount: 0,
            });
            setPermissions({
              canViewStudents: false,
              canViewTeachers: false,
              canViewRooms: false,
              canViewActivities: false,
              canViewGroups: false,
              canViewRoles: false,
              canViewDevices: false,
              canViewPermissions: false,
            });
          } else {
            console.error("Failed to fetch counts:", response.status);
          }
        }
      } catch (error) {
        console.error('Error fetching counts:', error);
      } finally {
        setCountsLoading(false);
      }
    };

    if (session?.user) {
      void fetchCounts();
    }
  }, [session]);

  if (status === "loading") {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-8 w-8 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (!session?.user) {
    redirect("/");
  }

  const renderTabContent = () => {
    switch (activeTab) {
      case "data":
        return (
          <div className="grid gap-6 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
            {baseDataSections.map((section) => {
              // Check permissions for this section
              const permissionKey = `canView${section.id.charAt(0).toUpperCase() + section.id.slice(1)}` as keyof typeof permissions;
              if (!permissions?.[permissionKey]) {
                return null; // Don't render sections user doesn't have permission for
              }

              // Get the count for this section (special case for permissions)
              const countKey = section.id === 'permissions' ? 'permissionCount' : section.id;
              const count = counts[countKey as keyof typeof counts] ?? 0;
              const countText = countsLoading ? "Lade..." : `${count} ${count === 1 ? 'Eintrag' : 'Einträge'}`;

              // Determine gradient colors for overlays based on section color
              const getOverlayColors = (colorClass: string) => {
                if (colorClass.includes('[#5080D8]')) return 'from-blue-50/80 to-cyan-100/80';
                if (colorClass.includes('[#F78C10]')) return 'from-orange-50/80 to-amber-100/80';
                if (colorClass.includes('[#83CD2D]')) return 'from-green-50/80 to-lime-100/80';
                if (colorClass.includes('[#FF3130]')) return 'from-red-50/80 to-rose-100/80';
                if (colorClass.includes('purple')) return 'from-purple-50/80 to-violet-100/80';
                if (colorClass.includes('indigo')) return 'from-indigo-50/80 to-blue-100/80';
                if (colorClass.includes('amber')) return 'from-amber-50/80 to-yellow-100/80';
                if (colorClass.includes('pink')) return 'from-pink-50/80 to-rose-100/80';
                return 'from-gray-50/80 to-slate-100/80';
              };

              const overlayColor = getOverlayColors(section.color);
              const ringColor = section.color.includes('[#5080D8]') ? 'group-hover:ring-blue-200/60' :
                              section.color.includes('[#F78C10]') ? 'group-hover:ring-orange-200/60' :
                              section.color.includes('[#83CD2D]') ? 'group-hover:ring-green-200/60' :
                              section.color.includes('[#FF3130]') ? 'group-hover:ring-red-200/60' :
                              section.color.includes('purple') ? 'group-hover:ring-purple-200/60' :
                              section.color.includes('indigo') ? 'group-hover:ring-indigo-200/60' :
                              section.color.includes('amber') ? 'group-hover:ring-amber-200/60' :
                              section.color.includes('pink') ? 'group-hover:ring-pink-200/60' :
                              'group-hover:ring-gray-200/60';

              const glowColor = section.color.includes('[#5080D8]') ? 'via-blue-100/30' :
                              section.color.includes('[#F78C10]') ? 'via-orange-100/30' :
                              section.color.includes('[#83CD2D]') ? 'via-green-100/30' :
                              section.color.includes('[#FF3130]') ? 'via-red-100/30' :
                              section.color.includes('purple') ? 'via-purple-100/30' :
                              section.color.includes('indigo') ? 'via-indigo-100/30' :
                              section.color.includes('amber') ? 'via-amber-100/30' :
                              section.color.includes('pink') ? 'via-pink-100/30' :
                              'via-gray-100/30';

              return (
                <Link
                  key={section.id}
                  href={section.href}
                  className="group relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 hover:scale-[1.01] hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] active:scale-[0.98] min-h-[44px] touch-manipulation"
                >
                  {/* Modern gradient overlay */}
                  <div className={`absolute inset-0 bg-gradient-to-br ${overlayColor} opacity-[0.03] rounded-3xl pointer-events-none`}></div>

                  {/* Subtle inner glow */}
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20 pointer-events-none"></div>

                  {/* Modern border highlight */}
                  <div className={`absolute inset-0 rounded-3xl ring-1 ring-white/20 ${ringColor} transition-all duration-300 pointer-events-none`}></div>

                  {/* Content */}
                  <div className="relative p-6">
                    {/* Icon and Count */}
                    <div className="flex items-start justify-between mb-4">
                      <div className={`rounded-2xl bg-gradient-to-br ${section.color} p-3 text-white shadow-lg group-hover:shadow-xl transition-all duration-300`}>
                        <Icon path={section.icon} className="h-6 w-6" />
                      </div>
                      <span className={`text-xs font-semibold px-3 py-1.5 rounded-full transition-all duration-200 ${countsLoading
                          ? "bg-gray-200 text-gray-400 animate-pulse"
                          : "bg-gray-100 text-gray-600"
                        }`}>
                        {countText}
                      </span>
                    </div>

                    {/* Title and Description */}
                    <h3 className="text-lg font-bold text-gray-900 mb-2 group-hover:text-gray-800 transition-colors">
                      {section.title}
                    </h3>
                    <p className="text-sm text-gray-600 line-clamp-2 mb-4">
                      {section.description}
                    </p>

                    {/* Arrow indicator */}
                    <div className="flex items-center text-gray-400 group-hover:text-gray-700 transition-colors">
                      <span className="text-sm font-medium">Verwalten</span>
                      <Icon
                        path="M9 5l7 7-7 7"
                        className="ml-2 h-4 w-4 transition-transform duration-200 group-hover:translate-x-1"
                      />
                    </div>
                  </div>

                  {/* Glowing border effect on hover */}
                  <div className={`absolute inset-0 rounded-3xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent ${glowColor} to-transparent pointer-events-none`}></div>
                </Link>
              );
            })}
          </div>
        );

      case "import":
        return (
          <div className="space-y-6">
            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">CSV-Datenimport</h3>
              <p className="text-sm text-gray-600 mb-4">Importieren Sie Daten aus CSV-Dateien in die Datenbank.</p>
              <Link
                href="/database/import"
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 text-sm font-medium text-white hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
              >
                <Icon path="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" className="h-5 w-5" />
                CSV Import starten
              </Link>
            </div>

            <div className="bg-white/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-100">
              <h3 className="text-base font-semibold text-gray-900 mb-3">Unterstützte Datentypen</h3>
              <ul className="space-y-2 text-sm text-gray-600">
                <li className="flex items-center gap-2">
                  <Icon path="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" className="h-5 w-5 text-green-600" />
                  Schülerdaten (Namen, Kontaktinformationen)
                </li>
                <li className="flex items-center gap-2">
                  <Icon path="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" className="h-5 w-5 text-green-600" />
                  Betreuer
                </li>
                <li className="flex items-center gap-2">
                  <Icon path="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" className="h-5 w-5 text-green-600" />
                  Räume und Ausstattung
                </li>
                <li className="flex items-center gap-2">
                  <Icon path="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" className="h-5 w-5 text-green-600" />
                  Aktivitäten und Gruppen
                </li>
              </ul>
            </div>
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="w-full">
      {/* Header - Show on mobile */}
      {isMobile && (
        <PageHeaderWithSearch
          title="Datenverwaltung"
        />
      )}

      {/* Tab Navigation - Desktop */}
      {!isMobile && (
        <div className="mb-6 inline-block bg-white/50 backdrop-blur-sm rounded-xl p-1 border border-gray-100">
          <div className="flex gap-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`
                  flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-all whitespace-nowrap
                  ${activeTab === tab.id
                    ? "bg-gray-900 text-white shadow-md"
                    : "text-gray-600 hover:bg-gray-200/80"
                  }
                `}
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={tab.icon} />
                </svg>
                <span>{tab.label}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Mobile Content - Always show data grid directly */}
      {isMobile && (
        <div className="min-h-[60vh]">
          <div className="grid gap-6 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
            {baseDataSections.map((section) => {
              // Check permissions for this section
              const permissionKey = `canView${section.id.charAt(0).toUpperCase() + section.id.slice(1)}` as keyof typeof permissions;
              if (!permissions?.[permissionKey]) {
                return null;
              }

              const countKey = section.id === 'permissions' ? 'permissionCount' : section.id;
              const count = counts[countKey as keyof typeof counts] ?? 0;
              const countText = countsLoading ? "Lade..." : `${count} ${count === 1 ? 'Eintrag' : 'Einträge'}`;

              const getOverlayColors = (colorClass: string) => {
                if (colorClass.includes('[#5080D8]')) return 'from-blue-50/80 to-cyan-100/80';
                if (colorClass.includes('[#F78C10]')) return 'from-orange-50/80 to-amber-100/80';
                if (colorClass.includes('[#83CD2D]')) return 'from-green-50/80 to-lime-100/80';
                if (colorClass.includes('[#FF3130]')) return 'from-red-50/80 to-rose-100/80';
                if (colorClass.includes('purple')) return 'from-purple-50/80 to-violet-100/80';
                if (colorClass.includes('indigo')) return 'from-indigo-50/80 to-blue-100/80';
                if (colorClass.includes('amber')) return 'from-amber-50/80 to-yellow-100/80';
                if (colorClass.includes('pink')) return 'from-pink-50/80 to-rose-100/80';
                return 'from-gray-50/80 to-slate-100/80';
              };

              const overlayColor = getOverlayColors(section.color);
              const ringColor = section.color.includes('[#5080D8]') ? 'group-hover:ring-blue-200/60' :
                              section.color.includes('[#F78C10]') ? 'group-hover:ring-orange-200/60' :
                              section.color.includes('[#83CD2D]') ? 'group-hover:ring-green-200/60' :
                              section.color.includes('[#FF3130]') ? 'group-hover:ring-red-200/60' :
                              section.color.includes('purple') ? 'group-hover:ring-purple-200/60' :
                              section.color.includes('indigo') ? 'group-hover:ring-indigo-200/60' :
                              section.color.includes('amber') ? 'group-hover:ring-amber-200/60' :
                              section.color.includes('pink') ? 'group-hover:ring-pink-200/60' :
                              'group-hover:ring-gray-200/60';

              const glowColor = section.color.includes('[#5080D8]') ? 'via-blue-100/30' :
                              section.color.includes('[#F78C10]') ? 'via-orange-100/30' :
                              section.color.includes('[#83CD2D]') ? 'via-green-100/30' :
                              section.color.includes('[#FF3130]') ? 'via-red-100/30' :
                              section.color.includes('purple') ? 'via-purple-100/30' :
                              section.color.includes('indigo') ? 'via-indigo-100/30' :
                              section.color.includes('amber') ? 'via-amber-100/30' :
                              section.color.includes('pink') ? 'via-pink-100/30' :
                              'via-gray-100/30';

              return (
                <Link
                  key={section.id}
                  href={section.href}
                  className="group relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 hover:scale-[1.01] hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] active:scale-[0.98] min-h-[44px] touch-manipulation"
                >
                  <div className={`absolute inset-0 bg-gradient-to-br ${overlayColor} opacity-[0.03] rounded-3xl pointer-events-none`}></div>
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20 pointer-events-none"></div>
                  <div className={`absolute inset-0 rounded-3xl ring-1 ring-white/20 ${ringColor} transition-all duration-300 pointer-events-none`}></div>

                  <div className="relative p-6">
                    <div className="flex items-start justify-between mb-4">
                      <div className={`rounded-2xl bg-gradient-to-br ${section.color} p-3 text-white shadow-lg group-hover:shadow-xl transition-all duration-300`}>
                        <Icon path={section.icon} className="h-6 w-6" />
                      </div>
                      <span className={`text-xs font-semibold px-3 py-1.5 rounded-full transition-all duration-200 ${countsLoading
                          ? "bg-gray-200 text-gray-400 animate-pulse"
                          : "bg-gray-100 text-gray-600"
                        }`}>
                        {countText}
                      </span>
                    </div>

                    <h3 className="text-lg font-bold text-gray-900 mb-2 group-hover:text-gray-800 transition-colors">
                      {section.title}
                    </h3>
                    <p className="text-sm text-gray-600 line-clamp-2 mb-4">
                      {section.description}
                    </p>

                    <div className="flex items-center text-gray-400 group-hover:text-gray-700 transition-colors">
                      <span className="text-sm font-medium">Verwalten</span>
                      <Icon
                        path="M9 5l7 7-7 7"
                        className="ml-2 h-4 w-4 transition-transform duration-200 group-hover:translate-x-1"
                      />
                    </div>
                  </div>

                  <div className={`absolute inset-0 rounded-3xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent ${glowColor} to-transparent pointer-events-none`}></div>
                </Link>
              );
            })}
          </div>
        </div>
      )}

      {/* Desktop Content - Full tab functionality */}
      {!isMobile && (
        <div className="min-h-[60vh]">
          {renderTabContent()}
        </div>
      )}
    </div>
  );
}

export default function DatabasePage() {
  return (
    <ResponsiveLayout>
      <Suspense
        fallback={
          <div className="flex min-h-[50vh] items-center justify-center">
            <div className="flex flex-col items-center gap-4">
              <div className="h-8 w-8 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
              <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        }
      >
        <DatabaseContent />
      </Suspense>
    </ResponsiveLayout>
  );
}
