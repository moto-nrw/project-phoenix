"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import Link from "~/components/ui/navigation-link";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { formatCount } from "~/lib/format-utils";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { StaffList } from "@/components/teachers/staff-list";
import {
  ExternalCaregiverCreateModal,
  ExternalInvitationModal,
  type ExternalInvitationTarget,
} from "~/components/staff/external-caregiver-dialogs";
import { useToast } from "~/contexts/ToastContext";
import type { Staff } from "~/lib/staff-api";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { hasPermission } from "~/lib/auth-utils";
import { canGrantExternalViewAccess } from "~/lib/external-caregiver-access";
import {
  EXTERNAL_CAREGIVER_LABEL,
  getRoleDisplayName,
} from "@/lib/auth-helpers";
import { staffDisplayName } from "~/lib/staff-helpers";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/components/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";
import { Modal } from "~/components/ui/modal";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { staffConcept } from "~/lib/concepts/staff";

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
  const [searchTerm, setSearchTerm] = useState(
    () => searchParams.get("search") ?? "",
  );
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
  // Externe Person anlegen und einladen (#4573): dasselbe Recht wie
  // POST /api/staff/externals und die Einladungsroute.
  const canCreateExternal = hasPermission(sessionData, "users:create");
  // „Nur ansehen“ vergibt die Rolle „Gast“ und braucht users:manage (ADR 0091).
  const canGrantView = canGrantExternalViewAccess(sessionData);
  const [showExternalModal, setShowExternalModal] = useState(false);
  const [inviteTarget, setInviteTarget] =
    useState<ExternalInvitationTarget | null>(null);
  const toast = useToast();

  const service = useMemo(() => createCrudService(teachersConfig), []);

  const {
    data: teachersData,
    isLoading: loading,
    error: teachersError,
    mutate: refreshTeachers,
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

    filtered.sort((a, b) =>
      staffDisplayName(a.name, a.first_name, a.last_name).localeCompare(
        staffDisplayName(b.name, b.first_name, b.last_name),
        "de",
      ),
    );

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

  // Der Rückweg trägt den tatsächlichen Suchzustand mit, damit „Zurück" die
  // gefilterte Personalübersicht wiederherstellt.
  const collectionReferrer = useMemo(() => {
    const query = new URLSearchParams(searchParams);
    if (searchTerm) query.set("search", searchTerm);
    else query.delete("search");
    const serialized = query.toString();
    return serialized ? `${COLLECTION_PATH}?${serialized}` : COLLECTION_PATH;
  }, [searchParams, searchTerm]);
  const objectHref = useCallback(
    (teacher: Teacher) => {
      return tenantPath(
        `/staff/${teacher.id}?tab=konto&from=${encodeURIComponent(collectionReferrer)}`,
      );
    },
    [collectionReferrer, tenantPath],
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
        // Externe Kräfte (#3823) bilden eine eigene Gruppe, mit jedem Zugang
        // (ADR 0091); sie trennt sie von Personal, dem nur die Rolle fehlt.
        if (teacher.is_external) {
          return {
            id: "__external__",
            title: EXTERNAL_CAREGIVER_LABEL,
            sortKey: "zzy",
          };
        }
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

  // Erfolg beim Anlegen oder Einladen einer externen Person (#4573). Eine
  // erneute Einladung ersetzt die alte; der Text sagt „neue Einladung“.
  // „Kein Zugang“ verschickt keine Einladung (ADR 0091).
  const handleExternalSaved = useCallback(
    (staff: Staff, outcome: "created" | "invited" | "reinvited") => {
      const name = staffDisplayName(
        staff.name,
        staff.firstName,
        staff.lastName,
      );
      const invited = staff.externalAccountStatus === "invitation_pending";
      const messages = {
        created: invited
          ? `${name} ist angelegt. Die Einladung ist per E-Mail unterwegs.`
          : `${name} ist angelegt.`,
        invited: `Die Einladung an ${name} ist unterwegs.`,
        reinvited: `Die neue Einladung an ${name} ist unterwegs.`,
      };
      toast.success(messages[outcome]);
      void refreshTeachers();
      setInvitationRefreshKey(Date.now());
    },
    [refreshTeachers, toast],
  );

  const handleInviteExternal = useCallback((teacher: Teacher) => {
    setInviteTarget({
      id: teacher.id,
      name: staffDisplayName(
        teacher.name,
        teacher.first_name,
        teacher.last_name,
      ),
      invitationPending:
        teacher.external_account_status === "invitation_pending",
    });
  }, []);

  const handleCloseInviteModal = useCallback(
    () => setShowInviteModal(false),
    [],
  );

  const openExternalCreate = useCallback(() => {
    setShowInviteModal(false);
    setShowExternalModal(true);
  }, []);

  // „+ Personal“ klappt auf, sobald es zwei Wege zum Anlegen gibt (#4573):
  // Personal einladen oder eine externe Person anlegen.
  const createMenuItems =
    canManageUsers && canCreateExternal
      ? [
          {
            label: "Personal einladen",
            onClick: () => setShowInviteModal(true),
            // Ziel der Einrichtungs-Tour (setup-tours.ts, Schritt „team“).
            tourTarget: "invite-create",
          },
          { label: "Externe Person anlegen", onClick: openExternalCreate },
        ]
      : undefined;

  const headerMenuItems = [
    ...(canImportOpeningBalances
      ? [
          {
            label: "Eröffnungssalden importieren",
            href: tenantPath("/database/personal/opening-balances"),
            onClick: () => undefined,
          },
        ]
      : []),
  ];

  const canShowList = !loading && filteredTeachers.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={error}
      overlays={
        <>
          {canManageUsers ? (
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
          ) : null}
          {canCreateExternal ? (
            <>
              <ExternalCaregiverCreateModal
                isOpen={showExternalModal}
                onClose={() => setShowExternalModal(false)}
                onSaved={(staff) => handleExternalSaved(staff, "created")}
                canGrantView={canGrantView}
              />
              <ExternalInvitationModal
                target={inviteTarget}
                canGrantView={canGrantView}
                onClose={() => setInviteTarget(null)}
                onInvited={(staff) =>
                  handleExternalSaved(
                    staff,
                    inviteTarget?.invitationPending ? "reinvited" : "invited",
                  )
                }
              />
            </>
          ) : null}
        </>
      }
      className="flex w-full flex-col"
      intro={{
        title: "Personal",
        description: statusLine,
        // Ohne Hülle: TenantPage ordnet jede Aktion selbst an. Auf dem Handy
        // bleibt das Kebab-Menü dann neben dem Titel statt in einer eigenen,
        // fast leeren Zeile.
        actions: (
          <>
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
            {headerMenuItems.length > 0 ? (
              <OverflowMenu
                ariaLabel="Weitere Aktionen"
                items={headerMenuItems}
              />
            ) : null}
            {canManageUsers || canCreateExternal ? (
              <DatabaseCreateAction
                label={canManageUsers ? "Personal" : "Externe Person"}
                ariaLabel={
                  canManageUsers
                    ? "Personal hinzufügen"
                    : "Externe Person anlegen"
                }
                onClick={
                  canManageUsers
                    ? () => setShowInviteModal(true)
                    : openExternalCreate
                }
                menuItems={createMenuItems}
              />
            ) : null}
          </>
        ),
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={staffConcept.icon}
                tone={staffConcept.tone}
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
      {/* Einladungen anlegen, erneut senden und zurückziehen verlangt
          users:manage. Wer es nicht hält, sieht den Block gar nicht: eine
          Leitungsrolle ohne das Recht bekommt hier keine Sperrkarte, die
          Personalseite selbst gehört ihr (#3469). */}
      {canManageUsers ? (
        <div className="mb-4">
          <PendingInvitationsList
            refreshKey={invitationRefreshKey}
            onRevoked={() => void refreshTeachers()}
          />
        </div>
      ) : null}

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
                icon={staffConcept.icon}
                tone={staffConcept.tone}
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
            onInviteExternal={
              canCreateExternal ? handleInviteExternal : undefined
            }
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
