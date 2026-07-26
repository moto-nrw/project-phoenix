"use client";

import { useState, useEffect, useRef } from "react";
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
import { AbwesenheitenTab } from "~/components/staff/abwesenheiten-tab";
import { ArbeitszeitmodellTab } from "~/components/staff/arbeitszeitmodell-tab";
import { UebersichtTab } from "~/components/staff/uebersicht-tab";
import { ZeiterfassungTab } from "~/components/staff/zeiterfassung-tab";
import { staffAbsenceService } from "~/lib/staff-api";
import { StaffDetailSkeleton } from "./page-skeleton";

// ─── Labels & constants ──────────────────────────────────────────────────────

// ─── Rich Header ─────────────────────────────────────────────────────────────

function StaffHeader({
  staff,
  onOpenMenu,
  menuOpen,
  menuButtonRef,
  menu,
}: {
  readonly staff: Staff;
  readonly onOpenMenu: () => void;
  readonly menuOpen: boolean;
  readonly menuButtonRef: React.RefObject<HTMLButtonElement | null>;
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
        <div className="relative">
          <button
            ref={menuButtonRef}
            type="button"
            onClick={onOpenMenu}
            aria-label="Weitere Aktionen"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            className="flex h-10 w-10 items-center justify-center rounded-full text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z"
              />
            </svg>
          </button>
          {menu}
        </div>
      </div>
    </div>
  );
}

// ─── Overflow (kebab) menu ───────────────────────────────────────────────────

interface MenuItem {
  readonly label: string;
  readonly onClick?: () => void;
  readonly disabled?: boolean;
  readonly destructive?: boolean;
}

function OverflowMenu({
  isOpen,
  onClose,
  items,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly items: ReadonlyArray<MenuItem | "divider">;
}) {
  if (!isOpen) return null;

  return (
    <>
      <button
        type="button"
        className="fixed inset-0 z-40 cursor-default"
        onClick={onClose}
        aria-label="Menue schliessen"
      />
      <div
        role="menu"
        className="absolute top-full right-0 z-50 mt-2 w-64 rounded-2xl border border-gray-200/50 bg-white/95 p-2 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md"
      >
        {items.map((item, idx) => {
          if (item === "divider") {
            return (
              <div
                key={`divider-${idx}`}
                className="my-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"
              />
            );
          }
          const baseClasses =
            "group flex w-full items-center rounded-xl px-3 py-2.5 text-left text-sm font-medium transition-all duration-150 ease-out";
          const stateClasses = item.disabled
            ? "cursor-not-allowed text-gray-300"
            : item.destructive
              ? "text-red-600 hover:bg-red-50 hover:text-red-700 active:bg-red-600 active:text-white"
              : "text-gray-700 hover:bg-gray-100 hover:text-gray-900 active:bg-gray-900 active:text-white";
          return (
            <button
              key={item.label}
              type="button"
              disabled={item.disabled}
              onClick={() => {
                if (item.disabled || !item.onClick) return;
                item.onClick();
                onClose();
              }}
              className={`${baseClasses} ${stateClasses}`}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    </>
  );
}

// ─── Placeholder tab (for disabled tabs) ─────────────────────────────────────

function PlaceholderTab({ title }: { readonly title: string }) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-8 text-center shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <p className="text-sm text-gray-400">{title} — kommt bald.</p>
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
  const canViewTimeTracking = canEdit || canManageTimeTracking;

  const {
    data: staff,
    isLoading,
    error,
  } = useSWRAuth<Staff>(`staff-detail-${staffId}`, () =>
    staffService.getStaffById(staffId),
  );

  // Counter for the "Abwesenheiten" tab — shows MA-Pending only.
  // The /staff dashboard inbox (Tranche 4c) will count across all staff.
  const { data: pendingForStaff } = useSWRAuth<number>(
    `staff-pending-absences-${staffId}`,
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

  const [menuOpen, setMenuOpen] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  // Radix Tabs auto-focuses the active TabsContent on mount, which the
  // browser then scrolls into view, leaving the staff header partially
  // clipped above the viewport. Snap to the top of the page after the
  // first paint so the avatar and name are always fully visible.
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [staffId]);

  // Close menu on Escape
  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [menuOpen]);

  if (sessionStatus === "loading" || isLoading) {
    return <StaffDetailSkeleton />;
  }

  if (!canViewTimeTracking) {
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

  const menuItems: ReadonlyArray<MenuItem | "divider"> = [
    {
      label: "PIN zuruecksetzen",
      disabled: true,
    },
    {
      label: "Passwort zuruecksetzen",
      disabled: true,
    },
    {
      label: "Abwesenheit eintragen",
      disabled: true,
    },
    "divider",
    {
      label: "Deaktivieren",
      disabled: true,
      destructive: true,
    },
    {
      label: "Loeschen",
      disabled: true,
      destructive: true,
    },
  ];

  return (
    <div className="-mt-1.5 w-full">
      {/* Rich Header with kebab trigger */}
      <StaffHeader
        staff={staff}
        onOpenMenu={() => setMenuOpen((v) => !v)}
        menuOpen={menuOpen}
        menuButtonRef={menuButtonRef}
        menu={
          <OverflowMenu
            isOpen={menuOpen}
            onClose={() => setMenuOpen(false)}
            items={menuItems}
          />
        }
      />

      {/* Tabs */}
      <Tabs
        defaultValue={canViewTimeTracking ? "uebersicht" : "abwesenheiten"}
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
          {canEdit ? (
            <>
              <TabsTrigger value="stammdaten" disabled>
                Stammdaten
              </TabsTrigger>
              <TabsTrigger value="dokumente" disabled>
                Dokumente
              </TabsTrigger>
            </>
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

        <TabsPrimitive.Content value="abwesenheiten">
          <AbwesenheitenTab
            staffId={staffId}
            canEdit={canEdit}
            canManageSickReports={canManageTimeTracking}
            staff={staff}
          />
        </TabsPrimitive.Content>

        {canEdit ? (
          <>
            <TabsPrimitive.Content value="stammdaten">
              <PlaceholderTab title="Stammdaten" />
            </TabsPrimitive.Content>

            <TabsPrimitive.Content value="dokumente">
              <PlaceholderTab title="Dokumente" />
            </TabsPrimitive.Content>
          </>
        ) : null}
      </Tabs>
    </div>
  );
}
