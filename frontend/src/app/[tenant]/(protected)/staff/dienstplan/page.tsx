"use client";

import { useMemo, useState } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";

import { Settings2 } from "lucide-react";

import { DienstplanWeekGrid } from "~/components/staff/dienstplan-week-grid";
import {
  ShiftEditModal,
  type ShiftEditMode,
} from "~/components/staff/shift-edit-modal";
import { ShiftTypeManageModal } from "~/components/staff/shift-type-manage-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { staffShiftService } from "~/lib/shift-api";
import {
  groupShiftsByStaffAndDate,
  type StaffScheduleAssignment,
  type StaffScheduleOverview,
  type StaffScheduleStaff,
  type StaffShift,
} from "~/lib/shift-helpers";
import { shiftTypeService } from "~/lib/shift-type-api";
import { indexShiftTypes, type ShiftType } from "~/lib/shift-type-helpers";
import { staffService, type Staff } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { getWeekNumber } from "~/lib/time-tracking-helpers";

import { DienstplanPageSkeleton } from "./page-skeleton";

// Admin week view for planned staff shifts (Dienstplan, #1376 core slice).
// One row per staff member, Mo–Fr columns, click-to-edit per cell. The
// planned shift end also drives the automatic checkout (#1798) when the
// tenant setting "Automatische Ausstempelung" is enabled.

function startOfWeek(d: Date): Date {
  const monday = new Date(d);
  const day = (monday.getDay() + 6) % 7; // Mon = 0
  monday.setDate(monday.getDate() - day);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

interface ModalState {
  mode: ShiftEditMode;
  staff: StaffScheduleStaff;
  date: string;
  shift: StaffShift | null;
}

function groupAssignmentsByStaffAndDate(
  assignments: readonly StaffScheduleAssignment[],
): Map<string, Map<string, StaffScheduleAssignment[]>> {
  const byStaff = new Map<string, Map<string, StaffScheduleAssignment[]>>();
  for (const assignment of assignments) {
    let byDate = byStaff.get(assignment.staffId);
    if (!byDate) {
      byDate = new Map<string, StaffScheduleAssignment[]>();
      byStaff.set(assignment.staffId, byDate);
    }
    const dayAssignments = byDate.get(assignment.date);
    if (dayAssignments) {
      dayAssignments.push(assignment);
    } else {
      byDate.set(assignment.date, [assignment]);
    }
  }
  return byStaff;
}

function DienstplanContent() {
  const { data: session, status: sessionStatus } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const router = useTenantRouter();
  const canEdit = isAdmin(session);
  const canUseAssignmentOverview =
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "users:read");
  const today = useBerlinToday();

  const [weekAnchor, setWeekAnchor] = useState<Date>(() =>
    startOfWeek(parseISODate(today)),
  );
  const [modal, setModal] = useState<ModalState | null>(null);
  const [manageOpen, setManageOpen] = useState(false);

  const weekDays = useMemo(() => {
    return Array.from({ length: 5 }, (_, i) => {
      const d = new Date(weekAnchor);
      d.setDate(d.getDate() + i);
      return toISODate(d);
    });
  }, [weekAnchor]);

  const weekFrom = weekDays[0] ?? "";
  const weekTo = weekDays[4] ?? "";

  const {
    data: overview,
    error: overviewError,
    isLoading: overviewLoading,
    mutate: mutateOverview,
  } = useSWRAuth<StaffScheduleOverview>(
    canUseAssignmentOverview
      ? `dienstplan-overview-${weekFrom}-${weekTo}`
      : null,
    () => staffShiftService.getOverview(weekFrom, weekTo),
  );

  const {
    data: legacyStaff,
    error: legacyStaffError,
    isLoading: legacyStaffLoading,
    mutate: mutateLegacyStaff,
  } = useSWRAuth<Staff[]>(
    canUseAssignmentOverview ? null : "dienstplan-staff",
    () => staffService.getAllStaff(),
  );

  const {
    data: legacyShifts,
    error: legacyShiftsError,
    isLoading: legacyShiftsLoading,
    mutate: mutateLegacyShifts,
  } = useSWRAuth<StaffShift[]>(
    canUseAssignmentOverview ? null : `dienstplan-shifts-${weekFrom}-${weekTo}`,
    () => staffShiftService.getShifts(weekFrom, weekTo),
  );

  const {
    data: shiftTypes,
    error: shiftTypesError,
    isLoading: shiftTypesLoading,
    mutate: mutateShiftTypes,
  } = useSWRAuth<ShiftType[]>("dienstplan-shift-types", () =>
    shiftTypeService.getShiftTypes(),
  );

  const typesById = useMemo(
    () => indexShiftTypes(shiftTypes ?? []),
    [shiftTypes],
  );

  const sortedStaff = useMemo(() => {
    const staff: StaffScheduleStaff[] = canUseAssignmentOverview
      ? (overview?.staff ?? [])
      : (legacyStaff ?? []).map((member) => ({
          id: member.id,
          firstName: member.firstName,
          lastName: member.lastName,
        }));
    return [...staff].sort((a, b) =>
      `${a.lastName} ${a.firstName}`.localeCompare(
        `${b.lastName} ${b.firstName}`,
        "de",
      ),
    );
  }, [canUseAssignmentOverview, legacyStaff, overview?.staff]);

  const shiftsByStaff = useMemo(
    () =>
      groupShiftsByStaffAndDate(
        canUseAssignmentOverview
          ? (overview?.shifts ?? [])
          : (legacyShifts ?? []),
      ),
    [canUseAssignmentOverview, legacyShifts, overview?.shifts],
  );

  const assignmentsByStaff = useMemo(
    () => groupAssignmentsByStaffAndDate(overview?.assignments ?? []),
    [overview?.assignments],
  );

  const isOnCurrentWeek =
    toISODate(startOfWeek(parseISODate(today))) === toISODate(weekAnchor);

  const weekLabel = useMemo(() => {
    const start = new Date(weekAnchor);
    const end = new Date(weekAnchor);
    end.setDate(end.getDate() + 4);
    const startLabel = start.toLocaleDateString("de-DE", {
      day: "numeric",
      month: "short",
    });
    const endLabel = end.toLocaleDateString("de-DE", {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
    return `KW ${getWeekNumber(weekAnchor)}: ${startLabel} bis ${endLabel}`;
  }, [weekAnchor]);

  const shiftWeek = (deltaDays: number) => {
    setWeekAnchor((prev) => {
      const next = new Date(prev);
      next.setDate(next.getDate() + deltaDays);
      return next;
    });
  };

  const scheduleError = canUseAssignmentOverview
    ? overviewError
    : (legacyStaffError ?? legacyShiftsError);
  const scheduleLoading = canUseAssignmentOverview
    ? overviewLoading
    : legacyStaffLoading || legacyShiftsLoading;
  const retryLoad = () => {
    void Promise.all(
      canUseAssignmentOverview
        ? [mutateOverview()]
        : [mutateLegacyStaff(), mutateLegacyShifts()],
    );
  };
  const mutateScheduleData = canUseAssignmentOverview
    ? mutateOverview
    : mutateLegacyShifts;

  if (sessionStatus === "loading") {
    return <DienstplanPageSkeleton />;
  }

  if (!canEdit) {
    router.replace("/staff");
    return <DienstplanPageSkeleton />;
  }

  return (
    <div className="space-y-4">
      {/* No in-page h1: the header breadcrumb already shows "Dienstplan"
          (breadcrumb-utils exactPageTitles), matching the app-wide pattern. */}
      <div className="flex justify-end">
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setManageOpen(true)}
          disabled={Boolean(shiftTypesError)}
        >
          <Settings2 className="mr-1.5 h-4 w-4" />
          Schichtarten verwalten
        </Button>
      </div>
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-col gap-3 sm:grid sm:grid-cols-3 sm:items-center">
          <p className="hidden text-xs text-gray-500 sm:block">
            Schichten und Betreuungsplan-Einsätze pro Mitarbeiter. Schichten
            lassen sich hier anlegen und bearbeiten.
          </p>
          <div className="flex min-w-0 items-center justify-center gap-2">
            <button
              type="button"
              onClick={() => shiftWeek(-7)}
              aria-label="Vorherige Woche"
              className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 19l-7-7 7-7"
                />
              </svg>
            </button>
            <h2 className="min-w-0 flex-1 text-center text-sm font-semibold text-gray-800 sm:min-w-[14rem]">
              {weekLabel}
            </h2>
            <button
              type="button"
              onClick={() => shiftWeek(7)}
              aria-label="Nächste Woche"
              className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>
          <div className="flex justify-center sm:justify-end">
            <button
              type="button"
              onClick={() => setWeekAnchor(startOfWeek(parseISODate(today)))}
              disabled={isOnCurrentWeek}
              className="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Diese Woche
            </button>
          </div>
        </div>
        {shiftTypesError && (
          <Alert
            type="warning"
            message="Schichtarten konnten nicht geladen werden. Der Dienstplan wird mit neutralen Schichtfarben angezeigt; die Schichtartenverwaltung ist vorübergehend deaktiviert."
          />
        )}
        {scheduleError ? (
          <div className="space-y-3">
            <Alert
              type="error"
              message="Der Dienstplan konnte nicht vollständig geladen werden. Bearbeiten ist deaktiviert, bis die Daten erfolgreich geladen wurden."
            />
            <button
              type="button"
              onClick={retryLoad}
              className="rounded-md border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Erneut laden
            </button>
          </div>
        ) : (
          <DienstplanWeekGrid
            staff={sortedStaff}
            shiftsByStaff={shiftsByStaff}
            assignmentsByStaff={assignmentsByStaff}
            weekDays={weekDays}
            todayIso={today}
            typesById={typesById}
            isLoading={scheduleLoading}
            onCellClick={(member, date, shift) =>
              setModal({
                mode: shift ? "edit" : "create",
                staff: member,
                date,
                shift,
              })
            }
          />
        )}
      </div>
      {modal && (
        <ShiftEditModal
          isOpen
          mode={modal.mode}
          staffId={modal.staff.id}
          staffName={`${modal.staff.firstName} ${modal.staff.lastName}`}
          date={modal.date}
          shift={modal.shift}
          shiftTypes={shiftTypes ?? []}
          onClose={() => setModal(null)}
          onSaved={() => mutateScheduleData()}
        />
      )}
      <ShiftTypeManageModal
        isOpen={manageOpen && !shiftTypesError}
        shiftTypes={shiftTypes ?? []}
        isLoading={shiftTypesLoading}
        loadError={Boolean(shiftTypesError)}
        onClose={() => setManageOpen(false)}
        onChanged={() => {
          mutateShiftTypes();
          mutateScheduleData();
        }}
      />
    </div>
  );
}

export default function DienstplanPage() {
  return <DienstplanContent />;
}
