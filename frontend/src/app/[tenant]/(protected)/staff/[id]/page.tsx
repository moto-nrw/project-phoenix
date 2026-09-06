"use client";

import { useEffect, useState } from "react";
import { useParams, redirect, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { staffService } from "~/lib/staff-api";
import type { Staff } from "~/lib/staff-api";
import {
  employmentTypeLabels,
  getStaffDisplayType,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { Avatar } from "~/components/ui/avatar";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { TenantPage, type TenantPageTab } from "~/components/ui/tenant-page";
import { AbwesenheitenTab } from "~/components/staff/abwesenheiten-tab";
import { ArbeitszeitmodellTab } from "~/components/staff/arbeitszeitmodell-tab";
import { DokumenteTab } from "~/components/staff/dokumente-tab";
import { KlassenTab } from "~/components/staff/klassen-tab";
import { StammdatenTab } from "~/components/staff/stammdaten-tab";
import { UebersichtTab } from "~/components/staff/uebersicht-tab";
import { ZeiterfassungTab } from "~/components/staff/zeiterfassung-tab";
import { staffAbsenceService } from "~/lib/staff-api";
import { isValidISODate } from "~/lib/date-helpers";
import { DetailSkeleton } from "~/components/ui/page-skeletons";
import { StaffDetailSkeleton } from "./page-skeleton";

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

  // Breadcrumb: Mitarbeiter / <Name>
  useSetBreadcrumb({
    staffName: staff ? `${staff.firstName} ${staff.lastName}` : undefined,
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

  if (!canViewTimeTracking && !canViewStammdaten && !canViewDocuments) {
    router.replace("/staff");
    return <StaffDetailSkeleton />;
  }

  if (!isLoading && (error || !staff)) {
    return (
      <TenantPage
        title="Mitarbeiter"
        back
        backHref="/staff"
        backLabel="Zurück zu den Mitarbeitenden"
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
  ];

  const defaultTab =
    requestedTab === "dokumente" && canViewDocuments
      ? "dokumente"
      : requestedTab === "zeiterfassung" && canViewTimeTracking
        ? "zeiterfassung"
        : canViewTimeTracking
          ? "uebersicht"
          : canViewStammdaten
            ? "stammdaten"
            : canViewDocuments
              ? "dokumente"
              : "abwesenheiten";
  const activeTab = selectedTab ?? defaultTab;

  return (
    <TenantPage
      // Der Entitätskopf IST die Kopfkarte: Avatar links, Name als Titel,
      // Statuszeile darunter, Status und Kebab als Aktionen.
      title={staff ? `${staff.firstName} ${staff.lastName}` : "Mitarbeiter"}
      stats={statusLine}
      statsLoading={!staff}
      // Rückweg ist die Mitarbeiterliste, nicht die Datenverwaltung.
      back
      backHref="/staff"
      backLabel="Zurück zu den Mitarbeitenden"
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
              StatusDotBadge und nicht StatusBadge. */}
          {staff && !staff.isLimitedProfile && locationStatus ? (
            <StatusDotBadge
              label={locationStatus.label}
              color={locationStatus.customBgColor}
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
