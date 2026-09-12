"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import Link from "~/components/ui/navigation-link";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { formatCount } from "~/lib/format-utils";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { StaffList } from "@/components/teachers/staff-list";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { RoleGuard } from "~/components/auth/role-guard";
import { hasPermission } from "~/lib/auth-utils";
import { getRoleDisplayName } from "@/lib/auth-helpers";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/components/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";
import { Modal } from "~/components/ui/modal";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

type StaffGroupingMode = "none" | "role";

const STAFF_GROUPING_DEFAULT: StaffGroupingMode = "role";

const STAFF_GROUPING_OPTIONS: { value: StaffGroupingMode; label: string }[] = [
  { value: "role", label: "Rolle" },
  { value: "none", label: "Keine" },
];

/** Die Sammlung dieser Seite, für den Rückweg aus der Personalakte (`?from=`). */
const COLLECTION_PATH = "/database/personal";

function parseStaffGrouping(value: string | null): StaffGroupingMode {
  if (value === "none") return value;
  return STAFF_GROUPING_DEFAULT;
}

// Search-match helper extracted so the page-level useMemo stays under
// the cognitive-complexity cap. Checks all teacher-display fields
// against a lowercased needle.
function matchesTeacherSearch(teacher: Teacher, searchLower: string): boolean {
  const haystacks = [
    teacher.first_name,
    teacher.last_name,
    teacher.name,
    teacher.role,
    teacher.account_role,
    teacher.specialization,
    teacher.email,
  ];
  return haystacks.some((h) => h?.toLowerCase().includes(searchLower) ?? false);
}

export default function TeachersPage() {
  return (
    <Suspense fallback={null}>
      <TeachersPageContent />
    </Suspense>
  );
}

/**
 * Personal (BAUARTEN-SPEC Bauart 1): die Sammlung mit Einladen, Import,
 * Gruppierung und Suche. Jede Zeile führt auf die Personalakte `/staff/[id]`,
 * die einzige Objektansicht einer Person (#3115). Bearbeiten, Löschen, Notizen
 * und die Kontoaktionen liegen dort im Reiter „Konto" und im Kebab der
 * Kopfkarte.
 */
function TeachersPageContent() {
  const tenantPath = useTenantAwarePath();
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const grouping = parseStaffGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [showInviteModal, setShowInviteModal] = useState(false);
  const [invitationRefreshKey, setInvitationRefreshKey] = useState<number>(
    Date.now(),
  );

  const { data: sessionData, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const canManageUsers = hasPermission(sessionData, "users:manage");
  // Seit #2906 erreicht die Seite auch, wer nur staff:manage oder
  // staff:stammdaten hat. Die beiden Import-Wege hängen an denselben
  // Berechtigungen wie ihre Backend-Routen: Personal-Import an users:create
  // (POST /api/import/teachers), Eröffnungssalden an der Zeitwirtschaft —
  // ohne diese Rechte antwortet das Backend mit 403.
  const canImportStaff = hasPermission(sessionData, "users:create");
  const canImportOpeningBalances = hasPermission(
    sessionData,
    "time_tracking:manage",
  );

  const service = useMemo(() => createCrudService(teachersConfig), []);

  const {
    data: teachersData,
    isLoading: loading,
    error: teachersError,
  } = useSWRAuth("database-teachers-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 1000 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = teachersError
    ? "Fehler beim Laden des Personals. Bitte versuchen Sie es später erneut."
    : null;

  // Statuszeile des Seitenkopfs aus der bereits geladenen Personalliste.
  const statusLine = useMemo(() => {
    const teachers = teachersData ?? [];
    const roles = new Set(
      teachers.map((t) => t.account_role?.trim()).filter(Boolean),
    ).size;
    const parts = [
      `${formatCount(teachers.length)} ${teachers.length === 1 ? "Person" : "Personen"}`,
    ];
    if (roles > 0) {
      parts.push(`${formatCount(roles)} ${roles === 1 ? "Rolle" : "Rollen"}`);
    }
    return parts.join(" · ");
  }, [teachersData]);

  const existingPositions = useMemo(() => {
    const teachers = teachersData ?? [];
    const positions = new Set<string>();
    for (const t of teachers) {
      if (t.role?.trim()) positions.add(t.role.trim());
    }
    return [...positions].sort((a, b) => a.localeCompare(b, "de"));
  }, [teachersData]);

  const filteredTeachers = useMemo(() => {
    const teachers = teachersData ?? [];
    let filtered = [...teachers];

    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter((teacher) =>
        matchesTeacherSearch(teacher, searchLower),
      );
    }

    filtered.sort((a, b) => {
      const nameA = a.name ?? `${a.first_name} ${a.last_name}`;
      const nameB = b.name ?? `${b.first_name} ${b.last_name}`;
      return nameA.localeCompare(nameB, "de");
    });

    return filtered;
  }, [teachersData, searchTerm]);

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];
    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }
    return filters;
  }, [searchTerm]);

  // Der Rückweg trägt die Gruppierung mit, damit „Zurück" aus der
  // Personalakte dieselbe Liste zeigt, die man verlassen hat.
  const objectHref = useCallback(
    (teacher: Teacher) => {
      const query = searchParams.toString();
      const from = query ? `${COLLECTION_PATH}?${query}` : COLLECTION_PATH;
      return tenantPath(
        `/staff/${teacher.id}?from=${encodeURIComponent(from)}`,
      );
    },
    [searchParams, tenantPath],
  );

  const handleGroupingChange = useCallback(
    (next: StaffGroupingMode) => {
      updateUrlParams({
        groupBy: next === STAFF_GROUPING_DEFAULT ? null : next,
      });
    },
    [updateUrlParams],
  );

  const groupers = useMemo<
    Partial<Record<StaffGroupingMode, Grouper<Teacher>>>
  >(
    () => ({
      role: (teacher) => {
        const role = teacher.account_role?.trim();
        if (!role) {
          return { id: "__no_role__", title: "Ohne Rolle", sortKey: "zzz" };
        }
        return { id: role, title: getRoleDisplayName(role) };
      },
    }),
    [],
  );

  const groupDefinitions = useGroupedItems(
    filteredTeachers,
    grouping,
    groupers,
    "Personal",
  );

  const handleCloseInviteModal = useCallback(
    () => setShowInviteModal(false),
    [],
  );

  const canShowList = !loading && filteredTeachers.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={error}
      overlays={
        canManageUsers ? (
          <Modal
            isOpen={showInviteModal}
            onClose={handleCloseInviteModal}
            title="Personal einladen"
          >
            <InvitationForm
              existingPositions={existingPositions}
              onCreated={() => {
                setInvitationRefreshKey(Date.now());
                setShowInviteModal(false);
              }}
            />
          </Modal>
        ) : null
      }
      className="flex w-full flex-col"
      intro={{
        title: "Personal",
        description: loading ? <Skeleton className="h-4 w-48" /> : statusLine,
        actions: (
          <div className="flex items-center gap-2">
            {!isMobile ? (
              <>
                <DatabaseGroupingToggle
                  value={grouping}
                  options={STAFF_GROUPING_OPTIONS}
                  onChange={handleGroupingChange}
                />
                {canImportStaff ? (
                  <Link
                    href={tenantPath("/database/personal/import")}
                    className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Importieren
                  </Link>
                ) : null}
              </>
            ) : null}
            {/* Zweiter Import-Weg (#2132): eigener Flow mit Stichtag und
                Begründung, deshalb im Menü statt als weiterer Button. */}
            {canImportOpeningBalances ? (
              <OverflowMenu
                ariaLabel="Weitere Import-Aktionen"
                items={[
                  {
                    label: "Eröffnungssalden importieren",
                    href: tenantPath("/database/personal/opening-balances"),
                    onClick: () => undefined,
                  },
                ]}
              />
            ) : null}
            {canManageUsers ? (
              <DatabaseCreateAction
                label="Personal"
                ariaLabel="Personal hinzufügen"
                onClick={() => setShowInviteModal(true)}
              />
            ) : null}
          </div>
        ),
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.staff.icon}
                tone={MOTO_CONCEPTS.staff.tone}
                size={20}
              />
            ),
            count: filteredTeachers.length,
            label: "Personal",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Personal suchen…",
          }}
          filters={[]}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
        />
      }
    >
      <RoleGuard variant="adminOnly" embedded>
        <div className="mb-4">
          <PendingInvitationsList refreshKey={invitationRefreshKey} />
        </div>
      </RoleGuard>

      {filteredTeachers.length === 0 ? (
        <SectionCard>
          <EmptyState
            title={
              searchTerm ? "Kein Personal gefunden" : "Kein Personal vorhanden"
            }
            description={
              searchTerm
                ? "Versuchen Sie andere Suchkriterien."
                : "Laden Sie die erste Person ein, damit sie sich anmelden kann."
            }
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.staff.icon}
                tone={MOTO_CONCEPTS.staff.tone}
                size={48}
              />
            }
          />
        </SectionCard>
      ) : canShowList ? (
        <div className="min-h-0 flex-1 pb-4">
          <StaffList
            groupDefinitions={groupDefinitions}
            objectHref={objectHref}
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
