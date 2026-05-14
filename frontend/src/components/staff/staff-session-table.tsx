"use client";

import { useMemo, useState } from "react";

import { Modal } from "~/components/ui/modal";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import {
  resolveTargetForDate,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";
import { formatDuration } from "~/lib/time-tracking-helpers";

import { formatSignedDuration } from "./staff-time-views";

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

// Day-row table for the Zeiterfassung tab. Industry-standard layout: one
// row per calendar day, comparing Soll, Check-in, Check-out, Pausen, Ist
// and Δ. Anomalies (auto-closed sessions, future ArbZG flags) are colour-
// coded so the admin can spot corrections at a glance.
//
// Click on a row opens a detail dialog. Until Tranche 1 (#1369) ships the
// admin-edit endpoint, the dialog is read-only with an explanatory note.
//
// MVP Limitation tracker:
//   - source/origin column (NFC/App/Admin) — Tranche 1
//   - inline edit with reason — Tranche 1
//   - ArbZG warning chips - Tranche 3 (#896)
export function StaffSessionTable({
  from,
  to,
  sessions,
  absences,
  schedule,
  today,
  isAdminView,
}: {
  readonly from: Date;
  readonly to: Date;
  readonly sessions: readonly StaffHistorySession[];
  readonly absences?: readonly StaffAbsenceRow[];
  readonly schedule: StaffSchedule | null;
  readonly today: Date;
  readonly isAdminView: boolean;
}) {
  const sessionsByDate = useMemo(() => {
    const map = new Map<string, StaffHistorySession>();
    for (const session of sessions) {
      // Normalise to YYYY-MM-DD; sessions can come back as either bare dates
      // or ISO timestamps depending on the backend serializer.
      map.set(session.date.slice(0, 10), session);
    }
    return map;
  }, [sessions]);

  // Absences arrive as date ranges; expand to a per-day lookup so each row in
  // the table can ask "is this date covered by Krank/Urlaub/etc?". Mirrors the
  // MA-side logic in time-tracking/page.tsx (absenceMap).
  const absencesByDate = useMemo(() => {
    const map = new Map<string, StaffAbsenceRow>();
    if (!absences) return map;
    for (const absence of absences) {
      const start = absence.date_start.slice(0, 10);
      const end = absence.date_end.slice(0, 10);
      const startDate = new Date(`${start}T00:00:00`);
      const endDate = new Date(`${end}T00:00:00`);
      const dayCount =
        Math.floor((endDate.getTime() - startDate.getTime()) / 86_400_000) + 1;
      for (let offset = 0; offset < dayCount; offset++) {
        const cursor = new Date(startDate);
        cursor.setDate(cursor.getDate() + offset);
        const key = toDateKey(cursor);
        // Existing entries win — the first absence for a day owns the badge.
        if (!map.has(key)) {
          map.set(key, absence);
        }
      }
    }
    return map;
  }, [absences]);

  const days = useMemo(() => {
    const result: Date[] = [];
    const start = new Date(from);
    const dayCount =
      Math.floor((to.getTime() - start.getTime()) / 86_400_000) + 1;
    for (let i = 0; i < dayCount; i++) {
      const d = new Date(start);
      d.setDate(d.getDate() + i);
      if (toIsoDayOfWeek(d) >= 5) continue;
      result.push(d);
    }
    return result;
  }, [from, to]);

  const [selected, setSelected] = useState<{
    day: Date;
    session: StaffHistorySession | undefined;
    absence: StaffAbsenceRow | undefined;
    target: number;
  } | null>(null);

  return (
    <>
      <div className="overflow-hidden rounded-2xl border border-gray-100">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            <tr>
              <th className="px-4 py-3">Datum</th>
              <th className="px-4 py-3">Tag</th>
              <th className="px-4 py-3 tabular-nums">Check-in</th>
              <th className="px-4 py-3 tabular-nums">Check-out</th>
              <th className="px-4 py-3 text-right tabular-nums">Pause</th>
              <th className="px-4 py-3 text-right tabular-nums">Soll</th>
              <th className="px-4 py-3 text-right tabular-nums">Ist</th>
              <th className="px-4 py-3 text-right tabular-nums">Saldo</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Quelle</th>
              <th className="px-4 py-3">Hinweis</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {days.map((day) => {
              const key = toDateKey(day);
              const dow = toIsoDayOfWeek(day);
              const session = sessionsByDate.get(key);
              const absence = absencesByDate.get(key);
              const target = schedule ? resolveTargetForDate(schedule, day) : 0;
              const isFuture = day > today;
              const isToday = sameDay(day, today);
              const ist = session?.net_minutes ?? 0;
              const delta = session && target > 0 ? ist - target : 0;
              const status = computeRowStatus(
                session,
                absence,
                target,
                isFuture,
              );
              return (
                <tr
                  key={key}
                  onClick={() => setSelected({ day, session, absence, target })}
                  className={`cursor-pointer transition-colors hover:bg-gray-50 ${
                    isToday ? "bg-amber-50/40" : ""
                  } ${isFuture ? "opacity-40" : ""}`}
                >
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {formatShortDate(day)}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{dayLabels[dow]}</td>
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {session?.check_in_time
                      ? formatTimeOnly(session.check_in_time)
                      : "–"}
                  </td>
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {session?.check_out_time ? (
                      formatTimeOnly(session.check_out_time)
                    ) : session ? (
                      <span className="inline-flex items-center rounded-full bg-[#83CD2D]/10 px-2 py-0.5 text-xs font-medium text-[#70b525]">
                        <span className="mr-1.5 h-1.5 w-1.5 animate-pulse rounded-full bg-[#83CD2D]" />
                        eingestempelt
                      </span>
                    ) : (
                      "–"
                    )}
                  </td>
                  <td className="px-4 py-3 text-right text-gray-500 tabular-nums">
                    {session ? formatDuration(session.break_minutes) : "–"}
                  </td>
                  <td className="px-4 py-3 text-right text-gray-500 tabular-nums">
                    {target > 0 ? formatDuration(target) : "–"}
                  </td>
                  <td className="px-4 py-3 text-right font-medium text-gray-700 tabular-nums">
                    {session ? formatDuration(ist) : "–"}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {session && target > 0 ? (
                      <span className={deltaClass(delta)}>
                        {formatSignedDuration(delta)}
                      </span>
                    ) : (
                      "–"
                    )}
                  </td>
                  <td className="px-4 py-3">
                    {status && <StatusBadge status={status} />}
                  </td>
                  <td className="px-4 py-3">
                    <SourceBadge session={session} />
                  </td>
                  <td className="px-4 py-3">
                    <HintBadges session={session} />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <SessionDetailDialog
        selected={selected}
        onClose={() => setSelected(null)}
        isAdminView={isAdminView}
      />
    </>
  );
}

// Row-Status drückt den Arbeitsmodus oder ein Fehlen aus. Vor Ort vs.
// Homeoffice ist Mitarbeiter-Intent. Krank/Urlaub/Fortbildung kommen aus
// active.staff_absences und überschreiben "Nicht erfasst", wenn der Tag durch
// eine Abwesenheit abgedeckt ist. "Nicht erfasst" bleibt für Werktage ohne
// Session UND ohne Absence (echte Lücke im Audit-Trail). Auto-Close- und
// Edit-Hinweise gehören NICHT hier rein — die fließen in die Quelle-Spalte
// (Issue #1368).
type RowStatus =
  | { kind: "present" }
  | { kind: "home-office" }
  | { kind: "absence"; absenceType: string }
  | { kind: "missing" };

function computeRowStatus(
  session: StaffHistorySession | undefined,
  absence: StaffAbsenceRow | undefined,
  target: number,
  isFuture: boolean,
): RowStatus | null {
  if (session) {
    if (session.status === "home_office") {
      return { kind: "home-office" };
    }
    return { kind: "present" };
  }
  // Absence wins over "missing" so an admin sees Krank/Urlaub instead of a
  // misleading "Nicht erfasst" badge (the MA-side already does this).
  if (absence) {
    return { kind: "absence", absenceType: absence.absence_type };
  }
  if (target > 0 && !isFuture) {
    return { kind: "missing" };
  }
  return null;
}

// German labels for the absence_type enum. Mirrors absenceTypeLabels in
// time-tracking-helpers.ts so the admin staff detail view shows the same
// wording as the MA-Sicht.
const absenceTypeLabel: Record<string, string> = {
  sick: "Krank",
  vacation: "Urlaub",
  training: "Fortbildung",
  other: "Abwesend",
};

// Tailwind classes per absence_type. Sick = red, vacation = blue,
// training = green, other = purple (matches absenceTypeColors in
// time-tracking-helpers.ts).
const absenceTypeBadge: Record<string, string> = {
  sick: "bg-red-100 text-red-800",
  vacation: "bg-blue-100 text-blue-800",
  training: "bg-green-100 text-green-800",
  other: "bg-purple-100 text-purple-800",
};

function StatusBadge({ status }: { readonly status: RowStatus }) {
  if (status.kind === "home-office") {
    return (
      <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#5080D8]">
        Homeoffice
      </span>
    );
  }
  if (status.kind === "present") {
    return (
      <span className="inline-flex items-center rounded-full bg-[#83CD2D]/10 px-2 py-0.5 text-xs font-medium text-[#70b525]">
        OGS
      </span>
    );
  }
  if (status.kind === "absence") {
    const label =
      absenceTypeLabel[status.absenceType] ?? absenceTypeLabel.other!;
    const classes =
      absenceTypeBadge[status.absenceType] ?? absenceTypeBadge.other!;
    return (
      <span
        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${classes}`}
      >
        {label}
      </span>
    );
  }
  return (
    <span className="inline-flex items-center rounded-full bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-500">
      Nicht erfasst
    </span>
  );
}

// Quelle-Badge ist der reine Origin der Session: über welchen Kanal sie
// entstanden ist (mutually exclusive). Korrekturen und Auto-Checkouts sind
// orthogonal und landen in der Hinweis-Spalte (HintBadges), damit z.B. eine
// NFC-Session mit nachträglicher Korrektur weiterhin als "NFC" erkennbar bleibt.
function SourceBadge({
  session,
}: {
  readonly session: StaffHistorySession | undefined;
}) {
  if (!session) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  if (session.source === "nfc") {
    return (
      <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#5080D8]">
        NFC
      </span>
    );
  }
  if (session.source === "unknown") {
    // Pre-Migration Legacy-Row (Tristan PR #1398).
    return <span className="text-xs text-gray-400">–</span>;
  }
  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
      App
    </span>
  );
}

// Hinweis-Spalte für orthogonale System- und Korrektur-Hinweise. Mehrere
// Pills können gleichzeitig sichtbar sein (z.B. NFC-Session mit Auto-Checkout
// UND nachträglicher Korrektur).
function HintBadges({
  session,
}: {
  readonly session: StaffHistorySession | undefined;
}) {
  if (!session) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  const pills: { key: string; label: string }[] = [];
  if ((session.edit_count ?? 0) > 0) {
    pills.push({ key: "edited", label: "Manuell korrigiert" });
  }
  if (session.auto_checked_out) {
    pills.push({ key: "auto-checkout", label: "Auto-Checkout" });
  }
  if (pills.length === 0) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {pills.map((pill) => (
        <span
          key={pill.key}
          className="inline-flex items-center rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700"
        >
          {pill.label}
        </span>
      ))}
    </div>
  );
}

function SessionDetailDialog({
  selected,
  onClose,
  isAdminView,
}: {
  readonly selected: {
    day: Date;
    session: StaffHistorySession | undefined;
    absence: StaffAbsenceRow | undefined;
    target: number;
  } | null;
  readonly onClose: () => void;
  readonly isAdminView: boolean;
}) {
  if (!selected) return null;
  const { day, session, absence, target } = selected;
  const ist = session?.net_minutes ?? 0;
  const delta = session && target > 0 ? ist - target : 0;
  return (
    <Modal isOpen onClose={onClose} title={formatLongDate(day)}>
      <div className="space-y-3 text-sm">
        {absence && !session && (
          <div className="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-700">
            <span className="font-semibold">
              {absenceTypeLabel[absence.absence_type] ??
                absenceTypeLabel.other!}
            </span>
            {absence.note && (
              <span className="ml-2 text-gray-500">— {absence.note}</span>
            )}
          </div>
        )}
        <DetailRow
          label="Check-in"
          value={
            session?.check_in_time ? formatTimeOnly(session.check_in_time) : "–"
          }
        />
        <DetailRow
          label="Check-out"
          value={
            session?.check_out_time
              ? formatTimeOnly(session.check_out_time)
              : session
                ? "noch offen"
                : "–"
          }
        />
        <DetailRow
          label="Pause"
          value={session ? formatDuration(session.break_minutes) : "–"}
        />
        <DetailRow
          label="Soll"
          value={target > 0 ? formatDuration(target) : "–"}
        />
        <DetailRow
          label="Ist"
          value={session ? formatDuration(ist) : "–"}
          emphasize
        />
        <DetailRow
          label="Saldo"
          value={session && target > 0 ? formatSignedDuration(delta) : "–"}
        />
        {session && (
          <DetailRow
            label="Quelle"
            value={
              session.source === "nfc"
                ? "NFC (Kiosk)"
                : session.source === "unknown"
                  ? "—"
                  : "App / Web"
            }
          />
        )}
        {session?.status === "home_office" && (
          <DetailRow label="Modus" value="Homeoffice" />
        )}
        {session?.auto_checked_out && (
          <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
            Diese Session wurde automatisch beendet, weil das Auschecken
            vergessen wurde. Eine Korrektur durch die Leitung ist empfohlen.
          </p>
        )}
        {session?.edit_count !== undefined && session.edit_count > 0 && (
          <DetailRow
            label="Korrekturen"
            value={`${session.edit_count}× bearbeitet`}
          />
        )}
        <p className="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600">
          {isAdminView
            ? "Korrekturen für andere Mitarbeitende kommen mit Tranche 1 (Issue #1369). Bis dahin kann nur der Mitarbeiter selbst seine Sessions ändern."
            : "Eigene Sessions können in der Zeiterfassung-Seite editiert werden."}
        </p>
      </div>
    </Modal>
  );
}

function DetailRow({
  label,
  value,
  emphasize,
}: {
  readonly label: string;
  readonly value: string;
  readonly emphasize?: boolean;
}) {
  return (
    <div className="flex items-center justify-between border-b border-gray-100 pb-2 last:border-b-0">
      <span className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
        {label}
      </span>
      <span
        className={`tabular-nums ${
          emphasize ? "text-base font-bold text-gray-800" : "text-gray-700"
        }`}
      >
        {value}
      </span>
    </div>
  );
}

function deltaClass(delta: number): string {
  if (delta > 0) return "font-medium text-amber-600";
  if (delta < -15) return "font-medium text-red-600";
  if (delta < 0) return "font-medium text-gray-500";
  return "font-medium text-green-600";
}

function toDateKey(d: Date): string {
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function formatShortDate(d: Date): string {
  const dd = String(d.getDate()).padStart(2, "0");
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  return `${dd}.${mm}.`;
}

function formatLongDate(d: Date): string {
  return d.toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

function formatTimeOnly(iso: string): string {
  const date = new Date(iso);
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
}
