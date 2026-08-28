"use client";

import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";

const logger = createLogger({ component: "DatabasePage" });
import { redirect } from "next/navigation";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import useSWR from "swr";
import { ChevronRight } from "lucide-react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { DatabaseCardGridSkeleton } from "./page-skeleton";
import { formatCount } from "~/lib/format-utils";

import { useNFCEnabled } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface DataSection {
  id: string;
  title: string;
  description: string;
  href: string;
  concept: MotoConceptKey;
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

const baseDataSections: DataSection[] = [
  {
    id: "students",
    title: "Kinder",
    description: "Kinderdaten verwalten und bearbeiten",
    href: "/database/students",
    concept: "children",
  },
  {
    id: "teachers",
    title: "Personal",
    description: "Personaldaten und Zuordnungen verwalten",
    href: "/database/personal",
    concept: "staff",
  },
  {
    id: "rooms",
    title: "Räume",
    description: "Räume und Ausstattung verwalten",
    href: "/database/rooms",
    concept: "rooms",
  },
  {
    id: "activities",
    title: "Aktivitäten",
    description: "Aktivitäten und Zeitpläne verwalten",
    href: "/database/activities",
    concept: "activities",
  },
  {
    id: "groups",
    title: "Gruppen",
    description: "Gruppen und Kombinationen verwalten",
    href: "/database/groups",
    concept: "groups",
  },
  {
    id: "roles",
    title: "Rollen",
    description: "Benutzerrollen und Berechtigungen verwalten",
    href: "/database/roles",
    concept: "roles",
  },
  {
    id: "devices",
    title: "Geräte",
    description: "Terminals und IoT-Geräte verwalten",
    href: "/database/devices",
    concept: "devices",
  },
  {
    id: "permissions",
    title: "Berechtigungen",
    description: "Systemberechtigungen ansehen",
    href: "/database/permissions",
    concept: "permissions",
  },
  {
    id: "gradeTransitions",
    title: "Jahrgangswechsel",
    description:
      "Kinder zum Schuljahreswechsel in die nächste Klasse versetzen",
    href: "/database/grade-transitions",
    concept: "gradeTransitions",
    permissionKey: "canViewGradeTransitions",
    badge: "Schuljahr",
    cta: "Öffnen",
  },
  {
    id: "exports",
    title: "Exporte",
    description: "Kinder-, Geburtstags-, Notfall- und Raumlisten erstellen",
    href: "/database/exports",
    concept: "exports",
    // Every export on that page reads child data, so it rides on the same
    // visibility as the Kinder section rather than inventing a flag.
    permissionKey: "canViewStudents",
    badge: "Listen",
    cta: "Öffnen",
  },
];

const NFC_ONLY_SECTION_IDS = new Set(["activities", "devices"]);

/** Statuszeile des Seitenkopfs: die Bestände, die der Zugriff hergibt.
 *  Zahlen stammen aus /api/database/counts, das die Seite ohnehin lädt. */
function buildDatabaseStatusLine(counts: DatabaseCounts): string {
  const permissions = counts.permissions;
  const parts: string[] = [];
  if (permissions.canViewStudents) {
    parts.push(`${formatCount(counts.students)} Kinder`);
  }
  if (permissions.canViewTeachers) {
    parts.push(`${formatCount(counts.teachers)} Personen`);
  }
  if (permissions.canViewRooms) {
    parts.push(`${formatCount(counts.rooms)} Räume`);
  }
  if (permissions.canViewGroups) {
    parts.push(`${formatCount(counts.groups)} Gruppen`);
  }
  return parts.join(" · ");
}

function DatabaseContent() {
  const { data: session } = useSession();
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

  const showSkeleton = countsLoading && data === undefined;
  const statusLine = buildDatabaseStatusLine(counts);

  return (
    <TenantPage
      title="Datenverwaltung"
      stats={statusLine}
      statsLoading={showSkeleton}
    >
      {showSkeleton ? (
        <DatabaseCardGridSkeleton />
      ) : (
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
                (countsLoading ? "Lädt…" : `${count} ${entryLabel}`);
              const badgeLoading = section.badge === undefined && countsLoading;
              const concept = MOTO_CONCEPTS[section.concept];

              return (
                <TileCard
                  key={section.id}
                  href={tenantPath(section.href)}
                  padding="none"
                  className="min-h-[44px] touch-manipulation"
                >
                  <div className="relative p-4 sm:p-6">
                    <div className="mb-4 flex items-start justify-between">
                      <div data-testid={`database-section-icon-${section.id}`}>
                        <MotoDuotoneIcon
                          icon={concept.icon}
                          tone={concept.tone}
                          size={36}
                        />
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

                    <h3 className="mb-2 text-base font-bold text-gray-900">
                      {section.title}
                    </h3>
                    <p className="mb-4 line-clamp-2 text-sm text-gray-600">
                      {section.description}
                    </p>

                    <div className="flex items-center text-gray-400 transition-colors group-hover:text-gray-700">
                      <span className="text-sm font-medium">
                        {section.cta ?? "Verwalten"}
                      </span>
                      <ChevronRight
                        aria-hidden="true"
                        className="ml-2 h-4 w-4 transition-transform duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] group-hover:translate-x-0.5 motion-reduce:transition-none motion-reduce:group-hover:translate-x-0"
                      />
                    </div>
                  </div>
                </TileCard>
              );
            })}
          </div>
        </div>
      )}
    </TenantPage>
  );
}

export default function DatabasePage() {
  return <DatabaseContent />;
}
