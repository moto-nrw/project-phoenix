"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, redirect, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { Pencil, Trash2 } from "lucide-react";
import { useTenantRouter } from "~/lib/tenant-router";
import { resolveDetailReferrer } from "~/lib/tenant-path";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { staffService } from "~/lib/staff-api";
import type { Staff } from "~/lib/staff-api";
import type { Teacher } from "~/lib/teacher-api";
import { teachersConfig } from "~/components/database/configs/teachers.config";
import { createCrudService } from "~/lib/database/service-factory";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  employmentTypeLabels,
  getStaffDisplayType,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { Avatar } from "~/components/ui/avatar";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { TenantPage, type TenantPageTab } from "~/components/ui/tenant-page";
import { AbwesenheitenTab } from "~/components/staff/abwesenheiten-tab";
import { ArbeitszeitmodellTab } from "~/components/staff/arbeitszeitmodell-tab";
import { DokumenteTab } from "~/components/staff/dokumente-tab";
import { KlassenTab } from "~/components/staff/klassen-tab";
import { KontoTab } from "~/components/staff/konto-tab";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { RoleManagementModal } from "~/components/teachers/role-management-modal";
import { TeacherEditModal } from "~/components/teachers/teacher-edit-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { StammdatenTab } from "~/components/staff/stammdaten-tab";
import { UebersichtTab } from "~/components/staff/uebersicht-tab";
import { ZeiterfassungTab } from "~/components/staff/zeiterfassung-tab";
import { staffAbsenceService } from "~/lib/staff-api";
import { isValidISODate } from "~/lib/date-helpers";
import { DetailSkeleton } from "~/components/ui/page-skeletons";
import { StaffDetailSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "StaffDetailPage" });

/** SWR-Schlüssel des Personal-Datensatzes (Konto-Reiter, Bearbeiten). */
function staffRecordKey(staffId: string): string {
  return `staff-record-${staffId}`;
}

// ─── Main Page ───────────────────────────────────────────────────────────────

export default function StaffDetailContent() {
  const { data: session, status: sessionStatus } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const staffId = params.id as string;
  // Rückweg: die Sammlung, aus der man kam. Die Datenverwaltung verlinkt seit
  // #3115 hierher statt in ein eigenes Pane; sonst die Mitarbeiterliste.
  const referrer = resolveDetailReferrer(searchParams.get("from"), "/staff", [
    "/staff",
    "/database/personal",
  ]);
  const backLabel = referrer.startsWith("/database/personal")
    ? "Zurück zum Personal"
    : "Zurück zu den Mitarbeitenden";
  const { success: toastSuccess, error: toastError } = useToast();
  const tenantMutate = useTenantMutate();
  const recordService = useMemo(() => createCrudService(teachersConfig), []);
  // Effective admin: the backend grants everything to `admin:*` / `*:*`
  // holders regardless of the role name, so a custom role carrying the
  // wildcard must see the same admin-gated UI as the literal admin role
  // (mirrors the staff list).
  const canEdit = isAdmin(session) || hasPermission(session, "admin:*");
  const canManageTimeTracking = hasPermission(session, "time_tracking:manage");
  const canManageAbsences =
    canEdit ||
    canManageTimeTracking ||
    hasPermission(session, "vacation:approve");
  // Urlaubsanspruch (#2906): Spiegel des Backend-Gates auf
  // PUT /{id}/vacation/quota — time_tracking:manage. Bewusst nicht
  // staff:manage: wer nur das hält, sieht weder den Anspruch (GET ist
  // vacation:approve / time_tracking:manage) noch den Abwesenheiten-Reiter.
  const canEditVacationQuota = canEdit || canManageTimeTracking;
  const canManagePayrollSettings = hasPermission(session, "config:manage");
  const canViewTimeTracking = canEdit || canManageTimeTracking;
  const canEditStammdaten = hasPermission(session, "staff:stammdaten");
  // Deliberately NOT users:read or users:update: the sections carry HR-file
  // data (birthday, private address, contract terms). users:read is held by
  // everyone who may see the staff list at all, users:update by the
  // Betreuer-Standardrolle for the child data (#2906) — mirrors the backend
  // route gate.
  const canViewStammdatenSections =
    canEdit || canManageTimeTracking || canEditStammdaten;
  // Klassen-Zuweisung (#1772): Spiegel der Backend-Gates — Lesen users:read,
  // Ersetzen users:manage (hasPermission ist wildcard-aware, admin:* matcht).
  const canViewKlassen = canEdit || hasPermission(session, "users:read");
  const canEditKlassen = canEdit || hasPermission(session, "users:manage");
  const canViewFinancial = hasPermission(session, "staff:financial");
  const canViewStammdaten = canViewStammdatenSections || canViewFinancial;
  // Dokumente (#1424): mirrors the backend route gate — any of the three
  // category permissions opens the tab; the backend then filters the list
  // to exactly the categories the caller may see.
  const canViewDocuments =
    canEdit ||
    hasPermission(session, "staff:documents") ||
    canViewFinancial ||
    hasPermission(session, "staff_documents:health");
  // Der Personal-Datensatz (Reiter „Konto", #3115): Spiegel des Backend-Gates
  // auf GET /api/staff/{id} — users:read, staff:manage, staff:stammdaten oder
  // time_tracking:manage. Das ist der Kreis, der vorher das Pane der
  // Datenverwaltung sah.
  const canViewRecord =
    canEdit ||
    canManageTimeTracking ||
    canEditStammdaten ||
    hasPermission(session, "users:read") ||
    hasPermission(session, "staff:manage");
  // Bearbeiten (Name, RFID-Karte, Position) geht über PUT /api/staff/{id}
  // und braucht staff:manage; Vorname, Nachname und Karte zusätzlich
  // users:update am Personen-Datensatz (#2906).
  const canManageStaffRecords = hasPermission(session, "staff:manage");
  const canEditPersonFields = hasPermission(session, "users:update");
  // Löschen und die Kontoaktionen hängen am Konto, nicht am Datensatz.
  const canDeleteStaff = hasPermission(session, "users:delete");
  const canManageUsers = hasPermission(session, "users:manage");
  const requestedTab = searchParams.get("tab");
  const requestedDate = searchParams.get("date");
  const initialTimeTrackingDate =
    requestedTab === "zeiterfassung" &&
    requestedDate !== null &&
    isValidISODate(requestedDate)
      ? requestedDate
      : undefined;

  // Vom Benutzer gewählter Reiter. Solange niemand gewechselt hat, entscheidet
  // die Berechtigung bzw. der Deep-Link — das kann erst gelten, wenn die
  // Sitzung aufgelöst ist, deshalb wird der Vorgabewert bei jedem Rendern neu
  // bestimmt statt einmalig im useState-Initialwert eingefroren.
  const [selectedTab, setSelectedTab] = useState<string | null>(null);

  const {
    data: staff,
    isLoading,
    error,
  } = useSWRAuth<Staff>(`staff-detail-${staffId}`, () =>
    canViewFinancial && !canViewStammdatenSections
      ? staffService.getFinancialProfile(staffId)
      : canViewDocuments && !canViewTimeTracking && !canViewStammdaten
        ? staffService.getDocumentProfile(staffId)
        : staffService.getStaffById(staffId),
  );

  // Der Datensatz für den Reiter „Konto" und das Bearbeiten: dieselbe
  // Abbildung wie das Register der Datenverwaltung (Systemrolle, RFID-Karte,
  // Notizen, Konto-ID), damit beide Seiten dieselben Felder kennen.
  const { data: record, error: recordError } = useSWRAuth<Teacher>(
    canViewRecord ? staffRecordKey(staffId) : null,
    () => recordService.getOne(staffId),
  );
  // Positionen der Schule als Vorschläge im Bearbeiten-Dialog; erst laden,
  // wenn der Dialog offen ist.
  const [showEditModal, setShowEditModal] = useState(false);
  const { data: staffDirectory } = useSWRAuth<Teacher[]>(
    showEditModal ? "database-teachers-list" : null,
    async () => {
      const data = await recordService.getList({ page: 1, pageSize: 1000 });
      return Array.isArray(data.data) ? data.data : [];
    },
  );
  const existingPositions = useMemo(() => {
    const positions = new Set<string>();
    for (const entry of Array.isArray(staffDirectory) ? staffDirectory : []) {
      if (entry.role?.trim()) positions.add(entry.role.trim());
    }
    return [...positions].sort((a, b) => a.localeCompare(b, "de"));
  }, [staffDirectory]);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [savingRecord, setSavingRecord] = useState(false);
  const [caregiverModalOpen, setCaregiverModalOpen] = useState(false);
  const [mfaModalOpen, setMfaModalOpen] = useState(false);
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const accessToken = session?.user?.token ?? "";

  const refreshRecord = useCallback(async () => {
    await Promise.all([
      tenantMutate(staffRecordKey(staffId)),
      tenantMutate(`staff-detail-${staffId}`),
      tenantMutate("database-teachers-list"),
    ]);
  }, [staffId, tenantMutate]);

  const handleEditRecord = useCallback(
    async (data: Partial<Teacher> & { password?: string }) => {
      try {
        setSavingRecord(true);
        await recordService.update(staffId, data);
        setShowEditModal(false);
        toastSuccess(
          getDbOperationMessage("update", teachersConfig.name.singular),
        );
        await refreshRecord();
      } catch (err) {
        logger.error("failed to update staff record", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      } finally {
        setSavingRecord(false);
      }
    },
    [recordService, refreshRecord, staffId, toastSuccess],
  );

  const handleUpdateNotes = useCallback(
    async (notes: string) => {
      await recordService.update(staffId, { staff_notes: notes });
      await refreshRecord();
    },
    [recordService, refreshRecord, staffId],
  );

  const handleDeleteRecord = useCallback(async () => {
    setDeleting(true);
    try {
      const deleteError = await recordService.delete(staffId);
      if (deleteError) {
        toastError(deleteError);
        return;
      }
      toastSuccess(
        getDbOperationMessage("delete", teachersConfig.name.singular),
      );
      await tenantMutate("database-teachers-list");
      setShowDeleteModal(false);
      router.push(referrer);
    } finally {
      setDeleting(false);
    }
  }, [
    recordService,
    referrer,
    router,
    staffId,
    tenantMutate,
    toastError,
    toastSuccess,
  ]);

  // Counter for the "Abwesenheiten" tab — shows MA-Pending only.
  // The /staff dashboard inbox (Tranche 4c) will count across all staff.
  const { data: pendingForStaff } = useSWRAuth<number>(
    canManageAbsences ? `staff-pending-absences-${staffId}` : null,
    async () => {
      const year = new Date().getFullYear();
      const rows = await staffAbsenceService.getAbsences(
        staffId,
        `${year}-01-01`,
        `${year}-12-31`,
      );
      return rows.filter(
        (r) => r.status === "requested" || r.status === "question",
      ).length;
    },
  );
  const pendingCount = pendingForStaff ?? 0;

  // Breadcrumb: Mitarbeiter / <Name>, aus dem Register heraus
  // Datenverwaltung / Personal / <Name>.
  useSetBreadcrumb({
    staffName: staff ? `${staff.firstName} ${staff.lastName}` : undefined,
    referrerPage: referrer,
  });

  // Radix Tabs auto-focuses the active TabsContent on mount, which the
  // browser then scrolls into view, leaving the staff header partially
  // clipped above the viewport. Snap to the top of the page after the
  // first paint so the avatar and name are always fully visible.
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [staffId]);

  // The tab set is permission-gated, and permissions come from the session,
  // not from the staff fetch below — so until the session resolves we don't
  // know which tabs to render at all and fall back to the full-page
  // skeleton. Once it resolves, the real tab bar renders immediately; only
  // the header (name/status, data-bound) and the active tab's content
  // (needs the fetched `staff`) stay skeletons while `isLoading`.
  if (sessionStatus === "loading") {
    return <StaffDetailSkeleton />;
  }

  if (
    !canViewTimeTracking &&
    !canViewStammdaten &&
    !canViewDocuments &&
    !canViewRecord
  ) {
    router.replace(referrer);
    return <StaffDetailSkeleton />;
  }

  if (!isLoading && (error || !staff)) {
    return (
      <TenantPage
        title="Mitarbeiter"
        back
        backHref={referrer}
        backLabel={backLabel}
        empty={{
          title: "Mitarbeiter konnte nicht geladen werden.",
          description:
            "Bitte laden Sie die Seite neu. Bleibt der Fehler bestehen, existiert die Person möglicherweise nicht mehr.",
        }}
      />
    );
  }

  const locationStatus = staff ? getStaffLocationStatus(staff) : null;
  const displayType = staff ? getStaffDisplayType(staff) : "";
  const employmentLabel = staff?.employmentType
    ? (employmentTypeLabels[staff.employmentType] ?? staff.employmentType)
    : null;
  // Statuszeile der Kopfkarte: echte Angaben zur Person, keine Erklärzeile.
  const statusLine = staff
    ? [
        staff.specialization ?? (displayType || null),
        employmentLabel,
        staff.email,
        staff.hasRfid ? "RFID zugewiesen" : null,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  // Seitenreiter: berechtigungsgesteuert, nicht datengesteuert. Die Flags
  // kommen aus der Sitzung, die hier bereits aufgelöst ist — die Reiterleiste
  // steht also sofort, auch während `staff` noch lädt.
  const tabItems: TenantPageTab[] = [
    ...(canViewTimeTracking
      ? [
          { value: "uebersicht", label: "Übersicht" },
          { value: "zeiterfassung", label: "Zeiterfassung" },
        ]
      : []),
    ...(canEdit
      ? [{ value: "arbeitszeitmodell", label: "Arbeitszeitmodell" }]
      : []),
    ...(canManageAbsences
      ? [
          {
            value: "abwesenheiten",
            label: "Abwesenheiten",
            badge: pendingCount,
          },
        ]
      : []),
    ...(canViewStammdaten
      ? [{ value: "stammdaten", label: "Stammdaten" }]
      : []),
    ...(canViewDocuments ? [{ value: "dokumente", label: "Dokumente" }] : []),
    ...(canViewKlassen ? [{ value: "klassen", label: "Klassen" }] : []),
    // Rolle, Zugang, Notizen und Kontoaktionen (#3115) — vorher das Pane der
    // Datenverwaltung, jetzt ein Reiter der einen Personalakte.
    ...(canViewRecord ? [{ value: "konto", label: "Konto" }] : []),
  ];

  const defaultTab =
    requestedTab === "dokumente" && canViewDocuments
      ? "dokumente"
      : requestedTab === "zeiterfassung" && canViewTimeTracking
        ? "zeiterfassung"
        : requestedTab === "konto" && canViewRecord
          ? "konto"
          : canViewTimeTracking
            ? "uebersicht"
            : canViewStammdaten
              ? "stammdaten"
              : canViewDocuments
                ? "dokumente"
                : canViewRecord
                  ? "konto"
                  : "abwesenheiten";
  const activeTab = selectedTab ?? defaultTab;

  // Aktionen am Datensatz im Kebab der Kopfkarte (Bauart 2 Regel 1).
  const recordMenuItems: OverflowMenuItem[] = [
    ...(canManageStaffRecords && record
      ? [
          {
            label: "Bearbeiten",
            icon: <Pencil className="size-4" aria-hidden />,
            onClick: () => setShowEditModal(true),
          },
        ]
      : []),
    ...(canDeleteStaff && record
      ? [
          {
            label: "Löschen",
            icon: <Trash2 className="size-4" aria-hidden />,
            destructive: true,
            onClick: () => setShowDeleteModal(true),
          },
        ]
      : []),
  ];
  const recordLabel = record
    ? `${record.first_name} ${record.last_name}`.trim()
    : "";
  const hasAccount = Boolean(record?.account_id);

  return (
    <TenantPage
      // Der Entitätskopf IST die Kopfkarte: Avatar links, Name als Titel,
      // Statuszeile darunter, Status und Kebab als Aktionen.
      title={staff ? `${staff.firstName} ${staff.lastName}` : "Mitarbeiter"}
      stats={statusLine}
      statsLoading={!staff}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={
        staff ? (
          // Kit-Avatar mit Initialen; der Name steht direkt daneben, das Bild
          // ist also rein schmückend.
          <Avatar
            name={`${staff.firstName} ${staff.lastName}`}
            size="lg"
            decorative
          />
        ) : undefined
      }
      actions={
        <>
          {/* Kein Glow, kein Pulsieren, dieselbe Entscheidung wie auf den
              Karten der Mitarbeiter-Liste. Die Farbe ist datengetrieben
              (LOCATION_COLORS über getStaffLocationStatus), deshalb
              StatusColorBadge und nicht StatusBadge. */}
          {staff && !staff.isLimitedProfile && locationStatus ? (
            <StatusColorBadge
              label={locationStatus.label}
              color={locationStatus.customBgColor}
            />
          ) : null}
          {recordMenuItems.length > 0 ? (
            <OverflowMenu
              ariaLabel="Weitere Aktionen"
              items={recordMenuItems}
            />
          ) : null}
        </>
      }
      tabs={{
        value: activeTab,
        onChange: setSelectedTab,
        items: tabItems,
        label: "Bereiche der Personalakte",
      }}
      overlays={
        record ? (
          <>
            <TeacherEditModal
              isOpen={showEditModal}
              onClose={() => setShowEditModal(false)}
              teacher={record}
              onSave={handleEditRecord}
              loading={savingRecord}
              existingPositions={existingPositions}
              canEditPersonFields={canEditPersonFields}
            />
            <ConfirmDeleteModal
              isOpen={showDeleteModal}
              onClose={() => setShowDeleteModal(false)}
              onConfirm={() => void handleDeleteRecord()}
              title="Personal löschen?"
              description={
                <>
                  Der Zugang wird deaktiviert und die Person aus allen Listen
                  entfernt. Vorhandene Einträge wie Anwesenheiten und
                  Zeiterfassung bleiben für die Historie erhalten. Die Person
                  kann jederzeit erneut eingeladen werden.
                </>
              }
              gate={{
                mode: "textConfirm",
                expected: recordLabel,
                inputId: "confirm-delete-staff-name",
                label: "Tippen Sie zur Bestätigung den Namen der Person:",
                preview: recordLabel,
                placeholder: "Vorname Nachname",
              }}
              loading={deleting}
              error=""
            />
            {hasAccount ? (
              <>
                <CaregiverCapabilityModal
                  isOpen={caregiverModalOpen}
                  onClose={() => setCaregiverModalOpen(false)}
                  scope="tenant"
                  accountId={record.account_id?.toString() ?? ""}
                  accountLabel={recordLabel}
                  onUpdated={refreshRecord}
                />
                {accessToken ? (
                  <MFAAdminOverrideModal
                    isOpen={mfaModalOpen}
                    onClose={() => setMfaModalOpen(false)}
                    bearerToken={accessToken}
                    accountId={record.account_id?.toString() ?? ""}
                    accountLabel={recordLabel}
                  />
                ) : null}
                <RoleManagementModal
                  isOpen={roleModalOpen}
                  onClose={() => setRoleModalOpen(false)}
                  accountId={record.account_id?.toString() ?? ""}
                  accountLabel={recordLabel}
                  onUpdated={refreshRecord}
                />
              </>
            ) : null}
          </>
        ) : null
      }
    >
      {staff ? (
        <>
          {activeTab === "uebersicht" && canViewTimeTracking && (
            <UebersichtTab staffId={staffId} />
          )}

          {activeTab === "zeiterfassung" && canViewTimeTracking && (
            <ZeiterfassungTab
              staffId={staffId}
              initialDate={initialTimeTrackingDate}
            />
          )}

          {activeTab === "arbeitszeitmodell" && canEdit && (
            <ArbeitszeitmodellTab staffId={staffId} canEdit={canEdit} />
          )}

          {activeTab === "abwesenheiten" && canManageAbsences && (
            <AbwesenheitenTab
              staffId={staffId}
              canEdit={canEdit}
              canEditQuota={canEditVacationQuota}
              canManageSickReports={canManageTimeTracking}
              staff={staff}
            />
          )}

          {activeTab === "stammdaten" && canViewStammdaten && (
            <StammdatenTab
              staffId={staffId}
              canManagePayroll={canManageTimeTracking}
              canManagePayrollSettings={canManagePayrollSettings}
              canViewSections={canViewStammdatenSections}
              canEditSections={canEditStammdaten}
              canViewFinancial={canViewFinancial}
            />
          )}

          {activeTab === "dokumente" && canViewDocuments && (
            <DokumenteTab staffId={staffId} />
          )}

          {/* Klassen-Zuweisung (#1772): scopt die Lehrkraft-Klassenansicht.
              Lesen mit users:read (wie die übrigen Staff-Detail-Reads),
              Ersetzen mit users:manage — beides erzwingt das Backend. */}
          {activeTab === "klassen" && canViewKlassen && (
            <KlassenTab staffId={staffId} canEdit={canEditKlassen} />
          )}

          {activeTab === "konto" && canViewRecord ? (
            record ? (
              <KontoTab
                teacher={record}
                onUpdateNotes={
                  canManageStaffRecords ? handleUpdateNotes : undefined
                }
                onManageRole={
                  canManageUsers && hasAccount
                    ? () => setRoleModalOpen(true)
                    : undefined
                }
                onManageMFA={
                  canManageUsers && hasAccount && accessToken
                    ? () => setMfaModalOpen(true)
                    : undefined
                }
                onManageCaregiver={
                  canManageUsers && hasAccount
                    ? () => setCaregiverModalOpen(true)
                    : undefined
                }
              />
            ) : recordError ? (
              <TenantPage
                title="Konto"
                error="Der Personal-Datensatz konnte nicht geladen werden."
              />
            ) : (
              <DetailSkeleton sections={2} fieldsPerSection={4} />
            )
          ) : null}
        </>
      ) : (
        // Der Inhalt des aktiven Reiters braucht die geladene Person (der
        // Abwesenheiten-Reiter bekommt sie direkt als Prop), er skelettiert
        // deshalb als Ganzes statt pro Reiter.
        <DetailSkeleton sections={2} fieldsPerSection={4} />
      )}
    </TenantPage>
  );
}
