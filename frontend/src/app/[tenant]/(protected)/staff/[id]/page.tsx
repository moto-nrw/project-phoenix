"use client";

import {
  useState,
  useMemo,
  useCallback,
  useEffect,
  useRef,
  Suspense,
} from "react";
import { useParams, redirect } from "next/navigation";
import { useSession } from "next-auth/react";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  staffService,
  staffScheduleService,
  staffHistoryService,
} from "~/lib/staff-api";
import type { Staff, StaffHistorySession } from "~/lib/staff-api";
import {
  getStaffDisplayType,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { formatDuration, getWeekDays } from "~/lib/time-tracking-helpers";
import { getInitials } from "~/lib/format-utils";
import { useSWRAuth } from "~/lib/swr";
import { useToast } from "~/contexts/ToastContext";
import { isAdmin } from "~/lib/auth-utils";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffDetailPage" });

// ─── Labels & constants ──────────────────────────────────────────────────────

const employmentTypeLabels: Record<string, string> = {
  full_time: "Vollzeit",
  part_time: "Teilzeit",
  minijob: "Minijob",
};

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

// ─── Rich Header ─────────────────────────────────────────────────────────────

function StaffHeader({
  staff,
  onOpenMenu,
  menuOpen,
  menuButtonRef,
}: {
  readonly staff: Staff;
  readonly onOpenMenu: () => void;
  readonly menuOpen: boolean;
  readonly menuButtonRef: React.RefObject<HTMLButtonElement | null>;
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
    <div className="mb-6 flex items-start justify-between gap-4">
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

// ─── Dienstplan Tab ──────────────────────────────────────────────────────────

function DienstplanTab({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const toast = useToast();
  const {
    data: schedule,
    isLoading: scheduleLoading,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  // Current week's actual hours from time-tracking history
  const weekDays = useMemo(() => getWeekDays(new Date()), []);
  const monday = weekDays[0];
  const sunday = weekDays[6];
  const fromDate = monday ? (monday.toISOString().split("T")[0] ?? "") : "";
  const toDate = sunday ? (sunday.toISOString().split("T")[0] ?? "") : "";

  const { data: historySessions, isLoading: historyLoading } = useSWRAuth<
    StaffHistorySession[]
  >(
    fromDate && toDate
      ? `staff-history-${staffId}-${fromDate}-${toDate}`
      : null,
    () => staffHistoryService.getHistory(staffId, fromDate, toDate),
  );

  const actualMinutesByDay = useMemo(() => {
    const map = new Map<number, number>();
    if (!historySessions) return map;
    for (const session of historySessions) {
      const date = new Date(session.date + "T00:00:00");
      const jsDay = date.getDay();
      const dayOfWeek = jsDay === 0 ? 6 : jsDay - 1;
      const current = map.get(dayOfWeek) ?? 0;
      map.set(dayOfWeek, current + (session.net_minutes ?? 0));
    }
    return map;
  }, [historySessions]);

  const isLoading = scheduleLoading || historyLoading;

  const [localEntries, setLocalEntries] = useState<
    Array<{ dayOfWeek: number; targetMinutes: number }>
  >([]);
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  const serverEntries = useMemo(() => {
    const result: Array<{ dayOfWeek: number; targetMinutes: number }> = [];
    for (let d = 0; d < 7; d++) {
      const existing = schedule?.entries.find((e) => e.dayOfWeek === d);
      result.push({
        dayOfWeek: d,
        targetMinutes: existing?.targetMinutes ?? 0,
      });
    }
    return result;
  }, [schedule]);

  const entries = isDirty ? localEntries : serverEntries;
  const weeklyTotal = entries.reduce((sum, e) => sum + e.targetMinutes, 0);
  const actualTotal = Array.from(actualMinutesByDay.values()).reduce(
    (a, b) => a + b,
    0,
  );

  const updateDay = useCallback(
    (dayOfWeek: number, minutes: number) => {
      const clamped = Math.max(0, Math.min(720, minutes));
      const next = (isDirty ? localEntries : serverEntries).map((e) =>
        e.dayOfWeek === dayOfWeek ? { ...e, targetMinutes: clamped } : e,
      );
      setLocalEntries(next);
      setIsDirty(true);
    },
    [isDirty, localEntries, serverEntries],
  );

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const nonZero = entries.filter((e) => e.targetMinutes > 0);
      await staffScheduleService.updateSchedule(staffId, nonZero);
      toast.success("Dienstplan gespeichert");
      setIsDirty(false);
      await mutateSchedule();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      toast.error("Fehler beim Speichern");
    } finally {
      setSaving(false);
    }
  }, [entries, staffId, toast, mutateSchedule]);

  if (isLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="mb-5 flex items-center justify-between">
        <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
          Wochenplan
        </h3>
        <div className="flex items-center gap-4 text-sm">
          <span className="font-bold text-gray-700">
            Soll: {formatDuration(weeklyTotal)}
          </span>
          {actualMinutesByDay.size > 0 && (
            <span className="text-gray-500">
              Ist:{" "}
              <span className="font-bold text-gray-700">
                {formatDuration(actualTotal)}
              </span>
            </span>
          )}
        </div>
      </div>

      {/* Column headers */}
      <div className="mb-2 flex items-center gap-4 text-xs font-medium tracking-wide text-gray-400 uppercase">
        <span className="w-8 shrink-0">Tag</span>
        <span className="w-[7.5rem] shrink-0 text-center">Soll</span>
        <span className="w-20 shrink-0 text-center">Ist</span>
        <span className="hidden flex-1 md:block" />
      </div>

      <div className="space-y-2">
        {entries.map((entry) => {
          const hours = Math.floor(entry.targetMinutes / 60);
          const mins = entry.targetMinutes % 60;
          const maxMinutes = 720;
          const pct = Math.min(100, (entry.targetMinutes / maxMinutes) * 100);
          const actual = actualMinutesByDay.get(entry.dayOfWeek);

          return (
            <div
              key={entry.dayOfWeek}
              className="flex items-center gap-4 rounded-lg py-1.5"
            >
              <span className="w-8 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[entry.dayOfWeek]}
              </span>

              {/* Soll: Hours + Minutes inputs */}
              <div className="flex w-[7.5rem] shrink-0 items-center justify-center gap-1">
                <input
                  type="number"
                  min={0}
                  max={12}
                  value={hours}
                  disabled={!canEdit}
                  onChange={(e) => {
                    const h = Math.max(
                      0,
                      Math.min(12, parseInt(e.target.value) || 0),
                    );
                    updateDay(entry.dayOfWeek, h * 60 + mins);
                  }}
                  className="w-12 rounded-lg border border-gray-200 px-1.5 py-1 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none disabled:bg-gray-50 disabled:text-gray-400"
                />
                <span className="text-xs text-gray-400">h</span>
                <select
                  value={mins}
                  disabled={!canEdit}
                  onChange={(e) => {
                    const m = parseInt(e.target.value) || 0;
                    updateDay(entry.dayOfWeek, hours * 60 + m);
                  }}
                  className="w-14 rounded-lg border border-gray-200 px-1 py-1 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none disabled:bg-gray-50 disabled:text-gray-400"
                >
                  <option value={0}>00</option>
                  <option value={15}>15</option>
                  <option value={30}>30</option>
                  <option value={45}>45</option>
                </select>
                <span className="text-xs text-gray-400">m</span>
              </div>

              {/* Ist: actual hours */}
              {(() => {
                if (actual === undefined) {
                  return (
                    <span className="w-20 shrink-0 text-center text-sm text-gray-300">
                      –
                    </span>
                  );
                }
                const isOver =
                  entry.targetMinutes > 0 && actual > entry.targetMinutes;
                const isUnder =
                  entry.targetMinutes > 0 && actual < entry.targetMinutes;
                const color = isOver
                  ? "text-amber-600"
                  : isUnder
                    ? "text-gray-500"
                    : "text-green-600";
                return (
                  <span
                    className={`w-20 shrink-0 text-center text-sm font-medium ${color}`}
                  >
                    {formatDuration(actual)}
                  </span>
                );
              })()}

              {/* Progress bar (Soll) */}
              <div className="hidden flex-1 md:block">
                <div className="h-2 overflow-hidden rounded-full bg-gray-100">
                  <div
                    className="h-full rounded-full bg-gray-300 transition-all"
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Save button (admin only) */}
      {canEdit && (
        <div className="mt-6 flex justify-end">
          <button
            type="button"
            onClick={handleSave}
            disabled={!isDirty || saving}
            className="rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {saving ? "Speichern..." : "Aenderungen speichern"}
          </button>
        </div>
      )}
    </div>
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

function StaffDetailContent() {
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

  const {
    data: staff,
    isLoading,
    error,
  } = useSWRAuth<Staff>(`staff-detail-${staffId}`, () =>
    staffService.getStaffById(staffId),
  );

  // Breadcrumb: Mitarbeiter / <Name>
  useSetBreadcrumb({
    staffName: staff ? `${staff.firstName} ${staff.lastName}` : undefined,
  });

  const [menuOpen, setMenuOpen] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

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
    return <Loading fullPage={false} />;
  }

  if (!canEdit) {
    router.replace("/staff");
    return <Loading fullPage={false} />;
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
      <div className="relative">
        <StaffHeader
          staff={staff}
          onOpenMenu={() => setMenuOpen((v) => !v)}
          menuOpen={menuOpen}
          menuButtonRef={menuButtonRef}
        />
        <OverflowMenu
          isOpen={menuOpen}
          onClose={() => setMenuOpen(false)}
          items={menuItems}
        />
      </div>

      {/* Tabs */}
      <Tabs defaultValue="dienstplan" className="w-full">
        <TabsList variant="line" className="mb-6 justify-start">
          <TabsTrigger value="uebersicht" disabled>
            Uebersicht
          </TabsTrigger>
          <TabsTrigger value="dienstplan">Dienstplan</TabsTrigger>
          <TabsTrigger value="zeiterfassung" disabled>
            Zeiterfassung
          </TabsTrigger>
          <TabsTrigger value="abwesenheiten" disabled>
            Abwesenheiten
          </TabsTrigger>
          <TabsTrigger value="stammdaten" disabled>
            Stammdaten
          </TabsTrigger>
          <TabsTrigger value="dokumente" disabled>
            Dokumente
          </TabsTrigger>
        </TabsList>

        <TabsPrimitive.Content value="uebersicht">
          <PlaceholderTab title="Uebersicht" />
        </TabsPrimitive.Content>

        <TabsPrimitive.Content value="dienstplan">
          <DienstplanTab staffId={staffId} canEdit={canEdit} />
        </TabsPrimitive.Content>

        <TabsPrimitive.Content value="zeiterfassung">
          <PlaceholderTab title="Zeiterfassung" />
        </TabsPrimitive.Content>

        <TabsPrimitive.Content value="abwesenheiten">
          <PlaceholderTab title="Abwesenheiten" />
        </TabsPrimitive.Content>

        <TabsPrimitive.Content value="stammdaten">
          <PlaceholderTab title="Stammdaten" />
        </TabsPrimitive.Content>

        <TabsPrimitive.Content value="dokumente">
          <PlaceholderTab title="Dokumente" />
        </TabsPrimitive.Content>
      </Tabs>
    </div>
  );
}

export default function StaffDetailPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <StaffDetailContent />
    </Suspense>
  );
}
