"use client";

import { Suspense, useMemo, useState } from "react";
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
import { Loading } from "~/components/ui/loading";
import { isAdmin } from "~/lib/auth-utils";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { staffShiftService } from "~/lib/shift-api";
import {
  groupShiftsByStaffAndDate,
  type StaffShift,
} from "~/lib/shift-helpers";
import { shiftTypeService } from "~/lib/shift-type-api";
import { indexShiftTypes, type ShiftType } from "~/lib/shift-type-helpers";
import { staffService, type Staff } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { getWeekNumber } from "~/lib/time-tracking-helpers";

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
  staff: Staff;
  date: string;
  shift: StaffShift | null;
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
    data: staff,
    error: staffError,
    isLoading: staffLoading,
    mutate: mutateStaff,
  } = useSWRAuth<Staff[]>("dienstplan-staff", () => staffService.getAllStaff());

  const {
    data: shifts,
    error: shiftsError,
    isLoading: shiftsLoading,
    mutate: mutateShifts,
  } = useSWRAuth<StaffShift[]>(`dienstplan-shifts-${weekFrom}-${weekTo}`, () =>
    staffShiftService.getShifts(weekFrom, weekTo),
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
    return [...(staff ?? [])].sort((a, b) =>
      `${a.lastName} ${a.firstName}`.localeCompare(
        `${b.lastName} ${b.firstName}`,
        "de",
      ),
    );
  }, [staff]);

  const shiftsByStaff = useMemo(
    () => groupShiftsByStaffAndDate(shifts ?? []),
    [shifts],
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

  const loadError = staffError ?? shiftsError ?? shiftTypesError;
  const retryLoad = () => {
    void Promise.all([mutateStaff(), mutateShifts(), mutateShiftTypes()]);
  };

  if (sessionStatus === "loading") {
    return <Loading fullPage={false} />;
  }

  if (!canEdit) {
    router.replace("/staff");
    return <Loading fullPage={false} />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-bold text-gray-900 sm:text-2xl">
          Dienstplan
        </h1>
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setManageOpen(true)}
        >
          <Settings2 className="mr-1.5 h-4 w-4" />
          Schichtarten verwalten
        </Button>
      </div>
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-col gap-3 sm:grid sm:grid-cols-3 sm:items-center">
          <p className="hidden text-xs text-gray-500 sm:block">
            Geplante Schichten pro Mitarbeiter. Klicke auf eine Zelle, um eine
            Schicht anzulegen oder zu bearbeiten.
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
        {loadError ? (
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
            weekDays={weekDays}
            todayIso={today}
            typesById={typesById}
            isLoading={staffLoading || shiftsLoading || shiftTypesLoading}
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
          onSaved={() => void mutateShifts()}
        />
      )}
      <ShiftTypeManageModal
        isOpen={manageOpen}
        shiftTypes={shiftTypes ?? []}
        onClose={() => setManageOpen(false)}
        onChanged={() => {
          void mutateShiftTypes();
          void mutateShifts();
        }}
      />
    </div>
  );
}

export default function DienstplanPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <DienstplanContent />
    </Suspense>
  );
}
