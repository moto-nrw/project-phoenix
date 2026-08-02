"use client";

import { useEffect } from "react";
import { useParams, redirect } from "next/navigation";
import { useSession } from "next-auth/react";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { staffService } from "~/lib/staff-api";
import type { Staff } from "~/lib/staff-api";
import {
  employmentTypeLabels,
  getStaffDisplayType,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { getInitials } from "~/lib/format-utils";
import { useSWRAuth } from "~/lib/swr";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { AbwesenheitenTab } from "~/components/staff/abwesenheiten-tab";
import { ArbeitszeitmodellTab } from "~/components/staff/arbeitszeitmodell-tab";
import { DokumenteTab } from "~/components/staff/dokumente-tab";
import { StammdatenTab } from "~/components/staff/stammdaten-tab";
import { UebersichtTab } from "~/components/staff/uebersicht-tab";
import { ZeiterfassungTab } from "~/components/staff/zeiterfassung-tab";
import { staffAbsenceService } from "~/lib/staff-api";
import { StaffDetailSkeleton } from "./page-skeleton";

// ─── Labels & constants ──────────────────────────────────────────────────────

// ─── Rich Header ─────────────────────────────────────────────────────────────

function StaffHeader({
  staff,
  menu,
}: {
  readonly staff: Staff;
  readonly menu: React.ReactNode;
}) {
  const locationStatus = getStaffLocationStatus(staff);
  const displayType = getStaffDisplayType(staff);
  const initials = getInitials(`${staff.firstName} ${staff.lastName}`);
  const employmentLabel = staff.employmentType
    ? (employmentTypeLabels[staff.employmentType] ?? staff.employmentType)
    : null;

  const position = staff.specialization ?? (displayType || null);
  const subtitleParts = [position, employmentLabel].filter(Boolean);
  const metaParts = [
    staff.email,
    staff.hasRfid ? "RFID zugewiesen" : null,
  ].filter(Boolean);

  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="flex min-w-0 flex-1 items-start gap-4">
        {/* Avatar - solid neutral gray, status is shown in badge */}
        <div className="flex h-14 w-14 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-base font-semibold text-gray-600 sm:h-16 sm:w-16 sm:text-lg">
          {initials}
        </div>

        {/* Eyebrow + Name + Subheading + meta */}
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
            Mitarbeiter-Profil
          </p>
          <h1 className="mt-1 truncate text-2xl font-bold text-gray-900 sm:text-3xl">
            {staff.firstName} {staff.lastName}
          </h1>
          {subtitleParts.length > 0 && (
            <p className="mt-2 truncate text-base font-medium text-gray-600">
              {subtitleParts.join(" · ")}
            </p>
          )}
          {metaParts.length > 0 && (
            <p className="mt-1 truncate text-xs text-gray-400">
              {metaParts.join(" · ")}
            </p>
          )}
        </div>
      </div>

      {/* Right side: Status badge + Kebab menu trigger */}
      <div className="flex flex-shrink-0 items-center gap-2">
        {!staff.isFinancialProfile ? (
          <span
            className={`inline-flex items-center rounded-full px-3 py-1.5 text-xs font-bold ${locationStatus.badgeColor}`}
            style={{
              backgroundColor: locationStatus.customBgColor,
              boxShadow: locationStatus.customShadow,
            }}
          >
            <span className="mr-1.5 h-1.5 w-1.5 animate-pulse rounded-full bg-white/80" />
            {locationStatus.label}
          </span>
        ) : null}
        {menu}
      </div>
    </div>
  );
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
  const staffId = params.id as string;
  const canEdit = isAdmin(session);
  const canManageTimeTracking = hasPermission(session, "time_tracking:manage");
  const canManageAbsences =
    canEdit ||
    canManageTimeTracking ||
    hasPermission(session, "vacation:approve");
  const canManagePayrollSettings = hasPermission(session, "config:manage");
  const canViewTimeTracking = canEdit || canManageTimeTracking;
  const canEditStammdaten = hasPermission(session, "users:update");
  // Deliberately NOT users:read: the sections carry HR-file data (birthday,
  // private address, contract terms), and users:read is held by everyone who
  // may see the staff list at all — mirrors the backend route gate.
  const canViewStammdatenSections =
    canEdit || canManageTimeTracking || canEditStammdaten;
  const canViewFinancial = hasPermission(session, "staff:financial");
  const canViewStammdaten = canViewStammdatenSections || canViewFinancial;
  // Dokumente (#1424): mirrors the backend route gate — any of the three
  // category permissions opens the tab; the backend then filters the list
  // to exactly the categories the caller may see.
  const canViewDocuments =
    canEdit ||
    canEditStammdaten ||
    canViewFinancial ||
    hasPermission(session, "staff_documents:health");

  const {
    data: staff,
    isLoading,
    error,
  } = useSWRAuth<Staff>(`staff-detail-${staffId}`, () =>
    canViewFinancial && !canViewStammdatenSections
      ? staffService.getFinancialProfile(staffId)
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

  if (sessionStatus === "loading" || isLoading) {
    return <StaffDetailSkeleton />;
  }

  if (!canViewTimeTracking && !canViewStammdaten) {
    router.replace("/staff");
    return <StaffDetailSkeleton />;
  }

  if (error || !staff) {
    return (
      <div className="py-12 text-center">
        <p className="text-gray-500">
          Mitarbeiter konnte nicht geladen werden.
        </p>
      </div>
    );
  }

  // All actions are placeholders until their flows land; noop keeps the
  // kit item shape (onClick is required) while disabled suppresses it anyway.
  const noop = () => undefined;
  const menuItems: readonly OverflowMenuEntry[] = [
    { label: "PIN zurücksetzen", onClick: noop, disabled: true },
    { label: "Passwort zurücksetzen", onClick: noop, disabled: true },
    { label: "Abwesenheit eintragen", onClick: noop, disabled: true },
    { kind: "separator" },
    { label: "Deaktivieren", onClick: noop, disabled: true, destructive: true },
    { label: "Löschen", onClick: noop, disabled: true, destructive: true },
  ];

  return (
    <div className="-mt-1.5 w-full">
      {/* Rich Header with kebab trigger */}
      <StaffHeader
        staff={staff}
        menu={<OverflowMenu items={menuItems} ariaLabel="Weitere Aktionen" />}
      />

      {/* Tabs */}
      <Tabs
        defaultValue={
          canViewTimeTracking
            ? "uebersicht"
            : canViewStammdaten
              ? "stammdaten"
              : "abwesenheiten"
        }
        className="w-full"
      >
        <TabsList
          variant="line"
          className="mb-6 w-full [scrollbar-width:none] justify-start overflow-x-auto pb-px [&::-webkit-scrollbar]:hidden"
        >
          {canViewTimeTracking ? (
            <TabsTrigger value="uebersicht">Übersicht</TabsTrigger>
          ) : null}
          {canViewTimeTracking ? (
            <TabsTrigger value="zeiterfassung">Zeiterfassung</TabsTrigger>
          ) : null}
          {canEdit ? (
            <TabsTrigger value="arbeitszeitmodell">
              Arbeitszeitmodell
            </TabsTrigger>
          ) : null}
          {canManageAbsences ? (
            <TabsTrigger value="abwesenheiten">
              <span className="inline-flex items-center gap-1.5">
                Abwesenheiten
                {pendingCount > 0 && (
                  <span className="inline-flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white">
                    {pendingCount > 99 ? "99+" : pendingCount}
                  </span>
                )}
              </span>
            </TabsTrigger>
          ) : null}
          {canViewStammdaten ? (
            <TabsTrigger value="stammdaten">Stammdaten</TabsTrigger>
          ) : null}
          {canViewDocuments ? (
            <TabsTrigger value="dokumente">Dokumente</TabsTrigger>
          ) : null}
        </TabsList>

        {canViewTimeTracking ? (
          <TabsPrimitive.Content value="uebersicht">
            <UebersichtTab staffId={staffId} />
          </TabsPrimitive.Content>
        ) : null}

        {canViewTimeTracking ? (
          <TabsPrimitive.Content value="zeiterfassung">
            <ZeiterfassungTab staffId={staffId} />
          </TabsPrimitive.Content>
        ) : null}

        {canEdit ? (
          <TabsPrimitive.Content value="arbeitszeitmodell">
            <ArbeitszeitmodellTab staffId={staffId} canEdit={canEdit} />
          </TabsPrimitive.Content>
        ) : null}

        {canManageAbsences ? (
          <TabsPrimitive.Content value="abwesenheiten">
            <AbwesenheitenTab
              staffId={staffId}
              canEdit={canEdit}
              canManageSickReports={canManageTimeTracking}
              staff={staff}
            />
          </TabsPrimitive.Content>
        ) : null}

        {canViewStammdaten ? (
          <TabsPrimitive.Content value="stammdaten">
            <StammdatenTab
              staffId={staffId}
              canManagePayroll={canManageTimeTracking}
              canManagePayrollSettings={canManagePayrollSettings}
              canViewSections={canViewStammdatenSections}
              canEditSections={canEditStammdaten}
              canViewFinancial={canViewFinancial}
            />
          </TabsPrimitive.Content>
        ) : null}

        {canViewDocuments ? (
          <TabsPrimitive.Content value="dokumente">
            <DokumenteTab staffId={staffId} />
          </TabsPrimitive.Content>
        ) : null}
      </Tabs>
    </div>
  );
}
