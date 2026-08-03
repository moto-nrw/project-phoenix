"use client";

import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";

const logger = createLogger({ component: "DatabasePage" });
import { redirect } from "next/navigation";
import Link from "next/link";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import useSWR from "swr";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { LOCATION_COLORS } from "~/lib/location-helper";

import { useNFCEnabled } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
// Icon component
const Icon: React.FC<{ path: string; className?: string }> = ({
  path,
  className,
}) => (
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

interface DataSection {
  id: string;
  title: string;
  description: string;
  href: string;
  icon: string;
  iconColor: string;
  /**
   * Permission flag from /api/database/counts gating this card. Defaults to the
   * `canView{Id}` flag derived from the section id; set it where the section has
   * no flag of its own.
   */
  permissionKey?: string;
  /** Replaces the entry-count badge for sections that count nothing. */
  badge?: string;
  /** Call to action on the card. Defaults to "Verwalten". */
  cta?: string;
}

interface DatabasePermissions {
  canViewStudents: boolean;
  canViewTeachers: boolean;
  canViewRooms: boolean;
  canViewActivities: boolean;
  canViewGroups: boolean;
  canViewRoles: boolean;
  canViewDevices: boolean;
  canViewPermissions: boolean;
  canViewTimetables: boolean;
  canViewGradeTransitions: boolean;
}

interface DatabaseCounts {
  students: number;
  teachers: number;
  rooms: number;
  activities: number;
  groups: number;
  roles: number;
  devices: number;
  permissionCount: number;
  permissions: DatabasePermissions;
}

const EMPTY_DATABASE_PERMISSIONS: DatabasePermissions = {
  canViewStudents: false,
  canViewTeachers: false,
  canViewRooms: false,
  canViewActivities: false,
  canViewGroups: false,
  canViewRoles: false,
  canViewDevices: false,
  canViewPermissions: false,
  canViewTimetables: false,
  canViewGradeTransitions: false,
};

const EMPTY_DATABASE_COUNTS: DatabaseCounts = {
  students: 0,
  teachers: 0,
  rooms: 0,
  activities: 0,
  groups: 0,
  roles: 0,
  devices: 0,
  permissionCount: 0,
  permissions: EMPTY_DATABASE_PERMISSIONS,
};

async function fetchDatabaseCounts(url: string): Promise<DatabaseCounts> {
  const response = await fetch(url);
  if (response.status === 401 || response.status === 403) {
    return EMPTY_DATABASE_COUNTS;
  }
  if (!response.ok) {
    throw new Error(`Database counts request failed (${response.status})`);
  }
  const result = (await response.json()) as {
    data: DatabaseCounts;
  };
  return result.data;
}

// Base data sections configuration with inline color styles
const baseDataSections: DataSection[] = [
  {
    id: "students",
    title: "Kinder",
    description: "Kinderdaten verwalten und bearbeiten",
    href: "/database/students",
    icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    iconColor: LOCATION_COLORS.OTHER_ROOM,
  },
  {
    id: "teachers",
    title: "Personal",
    description: "Personaldaten und Zuordnungen verwalten",
    href: "/database/personal",
    icon: "M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222",
    iconColor: LOCATION_COLORS.SCHOOLYARD,
  },
  {
    id: "rooms",
    title: "Räume",
    description: "Räume und Ausstattung verwalten",
    href: "/database/rooms",
    icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    iconColor: LOCATION_COLORS.TRANSIT,
  },
  {
    id: "activities",
    title: "Aktivitäten",
    description: "Aktivitäten und Zeitpläne verwalten",
    href: "/database/activities",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    iconColor: LOCATION_COLORS.HOME,
  },
  {
    id: "categories",
    title: "Kategorien",
    description: "Aktivitätskategorien anlegen, umbenennen und archivieren",
    href: "/database/categories",
    icon: "M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z",
    iconColor: LOCATION_COLORS.SICK,
    // Categories are activity Stammdaten and ride on the same visibility as
    // the Aktivitäten section rather than inventing a counts flag (#2131).
    permissionKey: "canViewActivities",
    badge: "Stammdaten",
    cta: "Verwalten",
  },
  {
    id: "groups",
    title: "Gruppen",
    description: "Gruppen und Kombinationen verwalten",
    href: "/database/groups",
    icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
    iconColor: LOCATION_COLORS.GROUP_ROOM,
  },
  {
    id: "roles",
    title: "Rollen",
    description: "Benutzerrollen und Berechtigungen verwalten",
    href: "/database/roles",
    icon: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
    iconColor: LOCATION_COLORS.EXCUSED,
  },
  {
    id: "devices",
    title: "Geräte",
    description: "Terminals und IoT-Geräte verwalten",
    href: "/database/devices",
    icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
    iconColor: LOCATION_COLORS.SICK,
  },
  {
    id: "permissions",
    title: "Berechtigungen",
    description: "Systemberechtigungen ansehen",
    href: "/database/permissions",
    icon: "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1 1 21 9z",
    iconColor: LOCATION_COLORS.HOME,
  },
  {
    id: "gradeTransitions",
    title: "Jahrgangswechsel",
    description:
      "Kinder zum Schuljahreswechsel in die nächste Klasse versetzen",
    href: "/database/grade-transitions",
    icon: "M13 7h8m0 0v8m0-8l-8 8-4-4-6 6",
    iconColor: LOCATION_COLORS.GROUP_ROOM,
    permissionKey: "canViewGradeTransitions",
    badge: "Schuljahr",
    cta: "Öffnen",
  },
  {
    id: "exports",
    title: "Exporte",
    description: "Kinder-, Geburtstags-, Notfall- und Raumlisten erstellen",
    href: "/database/exports",
    icon: "M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4",
    iconColor: LOCATION_COLORS.EXCUSED,
    // Every export on that page reads child data, so it rides on the same
    // visibility as the Kinder section rather than inventing a flag.
    permissionKey: "canViewStudents",
    badge: "Listen",
    cta: "Öffnen",
  },
];

const NFC_ONLY_SECTION_IDS = new Set(["activities", "devices"]);

function DatabaseContent() {
  const { data: session } = useSession();
  const isMobile = useIsMobile();
  const nfcEnabled = useNFCEnabled();
  const tenantPath = useTenantAwarePath();
  const { data, isLoading: countsLoading } = useSWR(
    session?.user ? "/api/database/counts" : null,
    fetchDatabaseCounts,
    {
      onError: (error: unknown) => {
        logger.error("failed to fetch counts", {
          error: error instanceof Error ? error.message : String(error),
        });
      },
    },
  );
  const counts = data ?? EMPTY_DATABASE_COUNTS;
  const permissions = counts.permissions;

  if (!session?.user) {
    redirect("/");
  }

  return (
    <div className="w-full">
      {/* Header - Show on mobile */}
      {isMobile && <PageHeaderWithSearch title="Datenverwaltung" />}

      {/* Data Sections Grid */}
      <div className="min-h-[60vh]">
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {baseDataSections.map((section) => {
            if (!nfcEnabled && NFC_ONLY_SECTION_IDS.has(section.id)) {
              return null;
            }

            // Check permissions for this section
            const permissionKey = (section.permissionKey ??
              `canView${section.id.charAt(0).toUpperCase() + section.id.slice(1)}`) as keyof typeof permissions;
            if (!permissions?.[permissionKey]) {
              return null;
            }

            const countKey =
              section.id === "permissions" ? "permissionCount" : section.id;
            const count = counts[countKey as keyof typeof counts] ?? 0;
            const entryLabel = count === 1 ? "Eintrag" : "Einträge";
            const countText =
              section.badge ??
              (countsLoading ? "Lade..." : `${count} ${entryLabel}`);
            const badgeLoading = section.badge === undefined && countsLoading;

            return (
              <Link
                key={section.id}
                href={tenantPath(section.href)}
                className="moto-content-surface moto-hover-elevated group relative min-h-[44px] touch-manipulation overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] active:shadow-[0_10px_26px_rgba(15,23,42,0.1)]"
              >
                <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-transparent transition-[box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] group-hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]"></div>

                <div className="relative p-6">
                  <div className="mb-4 flex items-start justify-between">
                    <div
                      data-testid={`database-section-icon-${section.id}`}
                      className="rounded-2xl p-3 text-white shadow-sm transition-[box-shadow,filter] duration-300 group-hover:shadow-md group-hover:brightness-95"
                      style={{ backgroundColor: section.iconColor }}
                    >
                      <Icon path={section.icon} className="h-6 w-6" />
                    </div>
                    <span
                      className={`rounded-full px-3 py-1.5 text-xs font-semibold transition-colors duration-200 ${
                        badgeLoading
                          ? "animate-pulse bg-gray-200 text-gray-400"
                          : "bg-gray-100 text-gray-600"
                      }`}
                    >
                      {countText}
                    </span>
                  </div>

                  <h3 className="mb-2 inline-block origin-left text-lg font-bold text-gray-900 transition-[color,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] group-hover:scale-[1.025] group-hover:text-gray-950 motion-reduce:transition-none motion-reduce:group-hover:scale-100">
                    {section.title}
                  </h3>
                  <p className="mb-4 line-clamp-2 text-sm text-gray-600">
                    {section.description}
                  </p>

                  <div className="flex items-center text-gray-400 transition-colors group-hover:text-gray-700">
                    <span className="text-sm font-medium">
                      {section.cta ?? "Verwalten"}
                    </span>
                    <Icon
                      path="M9 5l7 7-7 7"
                      className="ml-2 h-4 w-4 transition-transform duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] group-hover:translate-x-0.5 motion-reduce:transition-none motion-reduce:group-hover:translate-x-0"
                    />
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export default function DatabasePage() {
  return <DatabaseContent />;
}
