"use client";

import { isScopeBooked, STAFF_TAB_MODULES } from "~/lib/module-register";
import { useBookedModules } from "~/lib/tenant-context";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useParams, redirect, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { hasSessionAccess } from "~/lib/session-access";
import { KeyRound, Trash2, Users } from "lucide-react";
import { useTenantRouter } from "~/lib/tenant-router";
import { resolveDetailReferrer } from "~/lib/tenant-path";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { mapStaffRecord, staffService } from "~/lib/staff-api";
import type { BackendStaffResponse, Staff } from "~/lib/staff-api";
import { teacherService, type Teacher } from "~/lib/teacher-api";
import {
  mapTeacherResponse,
  teachersConfig,
} from "~/components/database/configs/teachers.config";
import { createCrudService } from "~/lib/database/service-factory";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import {
  employmentTypeLabels,
  getStaffDisplayType,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import {
  loadAccountRoleAssignment,
  replaceAccountRole,
  type AccountRoleAssignment,
} from "~/lib/account-role-assignment";
import { Avatar } from "~/components/ui/avatar";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { TenantPage, type TenantPageTab } from "~/components/ui/tenant-page";
import { FormerEmploymentDetails } from "~/components/staff/former-employment-details";
import { AbwesenheitenTab } from "~/components/staff/abwesenheiten-tab";
import { ArbeitszeitmodellTab } from "~/components/staff/arbeitszeitmodell-tab";
import { DokumenteTab } from "~/components/staff/dokumente-tab";
import { KlassenTab } from "~/components/staff/klassen-tab";
import {
  KontoRoleSaveError,
  KontoTab,
  type KontoDraft,
} from "~/components/staff/konto-tab";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { StammdatenTab } from "~/components/staff/stammdaten-tab";
import { UebersichtTab } from "~/components/staff/uebersicht-tab";
import { ZeiterfassungTab } from "~/components/staff/zeiterfassung-tab";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { staffAbsenceService } from "~/lib/staff-api";
import { isValidISODate } from "~/lib/date-helpers";
import { DetailSkeleton } from "~/components/ui/page-skeletons";
import { StaffDetailSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "StaffDetailPage" });

/**
 * SWR-Schlüssel des Personal-Datensatzes: die Rohdaten von GET /api/staff/{id}.
 * Kopf der Personalakte und Konto-Reiter leiten ihre Abbildungen daraus ab.
 */
function staffRecordKey(staffId: string): string {
  return `staff-record-${staffId}`;
}

/** SWR-Schlüssel der Systemrolle des Kontos (Konto-Bearbeiten, #3116). */
function roleAssignmentKey(accountId: string): string {
  return `staff-role-assignment-${accountId}`;
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
  const { success: toastSuccess } = useToast();
  // Ein Löschfehler steht im offenen Dialog; ein Toast läge hinter dem Modal
  // (#2514).
  const deleteErrors = useApiFormError();
  const { clear: clearDeleteError, show: showDeleteError } = deleteErrors;
  const handleDeleteRecordRef = useRef<() => Promise<void>>(
    async () => undefined,
  );
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
  const bookedModules = useBookedModules();
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
  // Bearbeiten (Name, Position, Notizen) geht über PUT /api/staff/{id} und
  // braucht staff:manage; Vorname und Nachname zusätzlich users:update am
  // Personen-Datensatz (#2906).
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

  // Wer nur das Finanz- oder Dokumentprofil sehen darf, bekommt dessen
  // schmalen Endpunkt; alle anderen den vollen Datensatz.
  const limitedProfile =
    canViewFinancial && !canViewStammdatenSections
      ? "financial"
      : canViewDocuments && !canViewTimeTracking && !canViewStammdaten
        ? "document"
        : null;
  const profileRequest = useSWRAuth<Staff>(
    limitedProfile ? `staff-detail-${staffId}` : null,
    () =>
      limitedProfile === "financial"
        ? staffService.getFinancialProfile(staffId)
        : staffService.getDocumentProfile(staffId),
  );
  const profile = profileRequest.data;

  // Der Datensatz kommt mit EINEM Abruf je Seite (#4993). Daraus entstehen
  // der Kopf der Personalakte und die Abbildung des Reiters „Konto": dieselbe
  // wie im Register der Datenverwaltung (Systemrolle, RFID-Karte, Notizen,
  // Konto-ID), damit beide Seiten dieselben Felder kennen. Neben einem
  // schmalen Profil lädt er nur für den Reiter „Konto".
  const recordRequest = useSWRAuth<BackendStaffResponse>(
    limitedProfile === null || (canViewRecord && profile && !profile.endedOn)
      ? staffRecordKey(staffId)
      : null,
    () => staffService.getStaffRecord(staffId),
  );
  const recordData = recordRequest.data;
  const fullStaff = useMemo(
    () => (recordData ? mapStaffRecord(recordData) : undefined),
    [recordData],
  );
  const record = useMemo<Teacher | undefined>(
    () =>
      canViewRecord && recordData ? mapTeacherResponse(recordData) : undefined,
    [canViewRecord, recordData],
  );
  const staffRequest = limitedProfile ? profileRequest : recordRequest;
  const staff = limitedProfile ? profile : fullStaff;
  const { isLoading, error } = staffRequest;
  const mutateStaff = staffRequest.mutate;
  // Ohne schmales Profil ist ein Fehler des Datensatzes der Ladefehler der
  // ganzen Seite, nicht des Reiters.
  const recordError = limitedProfile ? recordRequest.error : undefined;
  const mutateRecord = recordRequest.mutate;
  // Ladefehler stehen dort, wo die Daten fehlen, mit Wiederholen (#2514).
  // Ein 404 ist kein Ladefehler, sondern eine Person, die es nicht mehr gibt.
  const staffNotFound = error instanceof ApiError && error.status === 404;
  const staffLoadError = useSwrLoadError(
    staffNotFound ? undefined : error,
    "die Personalakte",
    mutateStaff,
  );
  const recordLoadError = useSwrLoadError(
    recordError,
    "das Konto",
    mutateRecord,
  );
  const accountId = record?.account_id ?? "";
  const hasAccount = accountId !== "";
  const canEditRole = canManageUsers && hasAccount;
  // Der Bearbeiten-Zustand des Konto-Reiters (#3116): Positionen der Schule
  // als Vorschläge und die Systemrolle des Kontos laden erst, wenn jemand
  // bearbeitet.
  const [kontoEditing, setKontoEditing] = useState(false);
  const { data: staffDirectory } = useSWRAuth<Teacher[]>(
    kontoEditing && canManageStaffRecords ? "database-teachers-list" : null,
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
  const { data: roleAssignment, error: roleAssignmentError } =
    useSWRAuth<AccountRoleAssignment>(
      kontoEditing && canEditRole ? roleAssignmentKey(accountId) : null,
      () => loadAccountRoleAssignment(accountId),
    );
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [caregiverModalOpen, setCaregiverModalOpen] = useState(false);
  const [mfaModalOpen, setMfaModalOpen] = useState(false);
  const hasAccess = hasSessionAccess(session);

  const refreshRecord = useCallback(async () => {
    await Promise.all([
      tenantMutate(staffRecordKey(staffId)),
      tenantMutate(`staff-detail-${staffId}`),
      tenantMutate("database-teachers-list"),
    ]);
  }, [staffId, tenantMutate]);

  // EIN Speichern für Name, Position, Notizen und Systemrolle (Bauart 2
  // Regel 4). Der Datensatz geht zuerst; scheitert danach der Rollenwechsel,
  // sagt die Meldung, was schon gespeichert ist.
  const handleSaveKonto = useCallback(
    async (draft: KontoDraft) => {
      const { role_id: targetRoleId, ...recordData } = draft;
      const hasRecordChanges = Object.keys(recordData).length > 0;
      if (hasRecordChanges) {
        try {
          await teacherService.updateTeacher(staffId, recordData);
        } catch (err) {
          logger.error("failed to update staff record", {
            staff_id: staffId,
            error: err instanceof Error ? err.message : String(err),
          });
          throw err;
        }
      }
      if (targetRoleId !== undefined) {
        try {
          await replaceAccountRole(accountId, targetRoleId);
        } catch (err) {
          logger.error("failed to update account role", {
            staff_id: staffId,
            account_id: accountId,
            target_role_id: targetRoleId,
            error: err instanceof Error ? err.message : String(err),
          });
          await refreshRecord();
          // Der Datensatz ist gespeichert; das sagt eine Erfolgsmeldung,
          // der Fehler im Formular nennt nur die Systemrolle.
          if (hasRecordChanges) {
            toastSuccess("Name, Position und Notizen sind gespeichert.");
          }
          throw new KontoRoleSaveError(err);
        }
        await tenantMutate(roleAssignmentKey(accountId));
      }
      toastSuccess("Die Änderungen sind gespeichert.");
      await refreshRecord();
    },
    [accountId, refreshRecord, staffId, tenantMutate, toastSuccess],
  );

  const handleDeleteRecord = useCallback(async () => {
    setDeleting(true);
    clearDeleteError();
    try {
      await teacherService.deleteTeacher(staffId);
      toastSuccess("Die Person ist gelöscht.");
      await tenantMutate("database-teachers-list");
      setShowDeleteModal(false);
      router.push(referrer);
    } catch (err) {
      logger.error("failed to delete staff record", {
        staff_id: staffId,
        error: err instanceof Error ? err.message : String(err),
      });
      await showDeleteError(err, {
        object: "die Person",
        retry: () => void handleDeleteRecordRef.current(),
      });
    } finally {
      setDeleting(false);
    }
  }, [
    clearDeleteError,
    referrer,
    router,
    showDeleteError,
    staffId,
    tenantMutate,
    toastSuccess,
  ]);
  useLayoutEffect(() => {
    handleDeleteRecordRef.current = handleDeleteRecord;
  });

  // Counter for the "Abwesenheiten" tab — shows MA-Pending only.
  // The /staff dashboard inbox (Tranche 4c) will count across all staff.
  const { data: pendingForStaff } = useSWRAuth<number>(
    canManageAbsences && staff && !staff.endedOn
      ? `staff-pending-absences-${staffId}`
      : null,
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
    return <StaffDetailSkeleton referrer={referrer} backLabel={backLabel} />;
  }

  if (
    !canViewTimeTracking &&
    !canViewStammdaten &&
    !canViewDocuments &&
    !canViewRecord
  ) {
    router.replace(referrer);
    return <StaffDetailSkeleton referrer={referrer} backLabel={backLabel} />;
  }

  if (!isLoading && error && !staffNotFound) {
    // Bis der Katalogtext geladen ist, bleibt das Skelett stehen.
    if (!staffLoadError)
      return <StaffDetailSkeleton referrer={referrer} backLabel={backLabel} />;
    return (
      <TenantPage
        title="Mitarbeiter"
        back
        backHref={referrer}
        backLabel={backLabel}
        error={staffLoadError}
      />
    );
  }

  if (!isLoading && (staffNotFound || !staff)) {
    return (
      <TenantPage
        title="Mitarbeiter"
        back
        backHref={referrer}
        backLabel={backLabel}
        empty={{
          title: "Diese Person gibt es nicht mehr.",
          description: "Sie wurde gelöscht oder der Link ist veraltet.",
        }}
      />
    );
  }

  const tabBooked = (tab: string) =>
    isScopeBooked(STAFF_TAB_MODULES[tab], bookedModules);

  if (staff?.endedOn) {
    return (
      <FormerEmploymentDetails
        staff={staff}
        referrer={referrer}
        backLabel={backLabel}
        canViewSections={canViewStammdatenSections && tabBooked("stammdaten")}
        canViewFinancial={canViewFinancial && tabBooked("stammdaten")}
        canViewTimeTracking={canViewTimeTracking && tabBooked("zeiterfassung")}
        canViewDocuments={canViewDocuments && tabBooked("dokumente")}
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

  // Reiter eines nicht gebuchten Moduls fehlen (#4894): Zeiterfassung,
  // Arbeitszeitmodell und Abwesenheiten gehören zur Zeiterfassung,
  // Stammdaten und Dokumente zur Personalakte (Modul-Register). Ein Reiter
  // trägt kein Schloss: die Navigation zeigt der Leitung das Modul.
  const showTimeTracking = canViewTimeTracking && tabBooked("zeiterfassung");
  const showWorkTimeModel = canEdit && tabBooked("arbeitszeitmodell");
  const showAbsences = canManageAbsences && tabBooked("abwesenheiten");
  const showStammdaten = canViewStammdaten && tabBooked("stammdaten");
  const showDocuments = canViewDocuments && tabBooked("dokumente");

  // Seitenreiter: berechtigungsgesteuert, nicht datengesteuert. Die Flags
  // kommen aus der Sitzung, die hier bereits aufgelöst ist — die Reiterleiste
  // steht also sofort, auch während `staff` noch lädt.
  const tabItems: TenantPageTab[] = [
    ...(showTimeTracking
      ? [
          { value: "uebersicht", label: "Übersicht" },
          { value: "zeiterfassung", label: "Zeiterfassung" },
        ]
      : []),
    ...(showWorkTimeModel
      ? [{ value: "arbeitszeitmodell", label: "Arbeitszeitmodell" }]
      : []),
    ...(showAbsences
      ? [
          {
            value: "abwesenheiten",
            label: "Abwesenheiten",
            badge: pendingCount,
          },
        ]
      : []),
    ...(showStammdaten ? [{ value: "stammdaten", label: "Stammdaten" }] : []),
    ...(showDocuments ? [{ value: "dokumente", label: "Dokumente" }] : []),
    ...(canViewKlassen ? [{ value: "klassen", label: "Klassen" }] : []),
    // Rolle, Zugang, Notizen und Kontoaktionen (#3115) — vorher das Pane der
    // Datenverwaltung, jetzt ein Reiter der einen Personalakte.
    ...(canViewRecord ? [{ value: "konto", label: "Konto" }] : []),
  ];

  const hasVisibleTab = (tab: string | null): tab is string =>
    tab !== null && tabItems.some((item) => item.value === tab);
  const defaultTab = hasVisibleTab(requestedTab)
    ? requestedTab
    : showTimeTracking
      ? "uebersicht"
      : showStammdaten
        ? "stammdaten"
        : showDocuments
          ? "dokumente"
          : canViewRecord
            ? "konto"
            : (tabItems[0]?.value ?? "konto");
  // Eine nachträglich entfernte Berechtigung oder Modulbuchung darf keinen
  // leeren Bereich hinterlassen. Der erste noch sichtbare Reiter übernimmt.
  const activeTab = hasVisibleTab(selectedTab) ? selectedTab : defaultTab;

  // Aktionen im Kebab der Kopfkarte (Bauart 2 Regel 1): die Kontoaktionen
  // mit eigenem Ablauf (#3116) und das Löschen. Bearbeitet wird im Reiter
  // „Konto", nicht von hier aus.
  const recordLabel = record
    ? `${record.first_name} ${record.last_name}`.trim()
    : "";
  const recordMenuItems: OverflowMenuItem[] = [
    ...(canManageUsers && hasAccount && hasAccess
      ? [
          {
            label: "Zwei-Faktor-Authentifizierung verwalten",
            icon: <KeyRound className="size-4" aria-hidden />,
            onClick: () => setMfaModalOpen(true),
          },
        ]
      : []),
    ...(canManageUsers && hasAccount
      ? [
          {
            label: "Betreuung verwalten",
            icon: <Users className="size-4" aria-hidden />,
            onClick: () => setCaregiverModalOpen(true),
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
          // Kit-Avatar: hochgeladenes Profilbild, sonst Initialen (#4432).
          // Der Name steht direkt daneben, das Bild ist also rein schmückend.
          <Avatar
            imageUrl={staff.avatarUrl}
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
            <ConfirmDeleteModal
              isOpen={showDeleteModal}
              onClose={() => {
                clearDeleteError();
                setShowDeleteModal(false);
              }}
              onConfirm={() => void handleDeleteRecord()}
              error={deleteErrors.error}
              title="Personal löschen?"
              description={
                <>
                  Der Zugang wird deaktiviert und die Person aus allen Listen
                  entfernt. Vorhandene Einträge wie Anwesenheiten und
                  Zeiterfassung bleiben bis zum Ende der Frist für die
                  Zeiterfassung erhalten. Danach löscht moto die Angaben der
                  Person. Die Person kann jederzeit erneut eingeladen werden.
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
            />
            {hasAccount ? (
              <>
                <CaregiverCapabilityModal
                  isOpen={caregiverModalOpen}
                  onClose={() => setCaregiverModalOpen(false)}
                  scope="tenant"
                  accountId={accountId}
                  accountLabel={recordLabel}
                  onUpdated={refreshRecord}
                />
                {hasAccess ? (
                  <MFAAdminOverrideModal
                    isOpen={mfaModalOpen}
                    onClose={() => setMfaModalOpen(false)}
                    accountId={accountId}
                    accountLabel={recordLabel}
                  />
                ) : null}
              </>
            ) : null}
          </>
        ) : null
      }
    >
      {staff ? (
        <>
          {activeTab === "uebersicht" && showTimeTracking && (
            <UebersichtTab staffId={staffId} />
          )}

          {activeTab === "zeiterfassung" && showTimeTracking && (
            <ZeiterfassungTab
              staffId={staffId}
              initialDate={initialTimeTrackingDate}
              staff={staff}
              canBookAbsences={canManageTimeTracking}
            />
          )}

          {activeTab === "arbeitszeitmodell" && showWorkTimeModel && (
            <ArbeitszeitmodellTab staffId={staffId} canEdit={canEdit} />
          )}

          {activeTab === "abwesenheiten" && showAbsences && (
            <AbwesenheitenTab
              staffId={staffId}
              canEdit={canEdit}
              canEditQuota={canEditVacationQuota}
              canManageSickReports={canManageTimeTracking}
              staff={staff}
            />
          )}

          {activeTab === "stammdaten" && showStammdaten && (
            <StammdatenTab
              staffId={staffId}
              canManagePayroll={canManageTimeTracking}
              canManagePayrollSettings={canManagePayrollSettings}
              canViewSections={canViewStammdatenSections}
              canEditSections={canEditStammdaten}
              canViewFinancial={canViewFinancial}
            />
          )}

          {activeTab === "dokumente" && showDocuments && (
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
                key={record.id}
                teacher={record}
                editing={
                  canManageStaffRecords || canEditRole
                    ? {
                        canEditPersonFields:
                          canManageStaffRecords && canEditPersonFields,
                        canEditStaffFields: canManageStaffRecords,
                        existingPositions,
                        canEditRole,
                        canDeleteStaff,
                        roleAssignment,
                        roleAssignmentError: Boolean(roleAssignmentError),
                        onEditingChange: setKontoEditing,
                        onSave: handleSaveKonto,
                      }
                    : undefined
                }
              />
            ) : recordError ? (
              <LoadErrorAlert error={recordLoadError} />
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
