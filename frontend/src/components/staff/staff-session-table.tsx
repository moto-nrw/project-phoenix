"use client";

import { useMemo, useState } from "react";

import { Modal } from "~/components/ui/modal";
import type { StaffHistorySession } from "~/lib/staff-api";
import { toIsoDayOfWeek } from "~/lib/staff-metrics-helpers";
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
  targetByDow,
  today,
  isAdminView,
}: {
  readonly from: Date;
  readonly to: Date;
  readonly sessions: readonly StaffHistorySession[];
  readonly targetByDow: Map<number, number>;
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

  const days = useMemo(() => {
    const result: Date[] = [];
    const start = new Date(from);
    const dayCount =
      Math.floor((to.getTime() - start.getTime()) / 86_400_000) + 1;
    for (let i = 0; i < dayCount; i++) {
      const d = new Date(start);
      d.setDate(d.getDate() + i);
      result.push(d);
    }
    return result;
  }, [from, to]);

  const [selected, setSelected] = useState<{
    day: Date;
    session: StaffHistorySession | undefined;
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
              <th className="px-4 py-3 text-right tabular-nums">Soll</th>
              <th className="px-4 py-3 tabular-nums">Check-in</th>
              <th className="px-4 py-3 tabular-nums">Check-out</th>
              <th className="px-4 py-3 text-right tabular-nums">Pause</th>
              <th className="px-4 py-3 text-right tabular-nums">Ist</th>
              <th className="px-4 py-3 text-right tabular-nums">Δ</th>
              <th className="px-4 py-3">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {days.map((day) => {
              const key = toDateKey(day);
              const dow = toIsoDayOfWeek(day);
              const session = sessionsByDate.get(key);
              const target = targetByDow.get(dow) ?? 0;
              const isWeekend = dow >= 5;
              const isFuture = day > today;
              const isToday = sameDay(day, today);
              const ist = session?.net_minutes ?? 0;
              const delta = session && target > 0 ? ist - target : 0;
              const status = computeRowStatus(session, target, isFuture);
              return (
                <tr
                  key={key}
                  onClick={() => setSelected({ day, session, target })}
                  className={`cursor-pointer transition-colors hover:bg-gray-50 ${
                    isToday
                      ? "bg-amber-50/40"
                      : isWeekend
                        ? "bg-gray-50/40"
                        : ""
                  } ${isFuture ? "opacity-40" : ""}`}
                >
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {formatShortDate(day)}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{dayLabels[dow]}</td>
                  <td className="px-4 py-3 text-right text-gray-500 tabular-nums">
                    {target > 0 ? formatDuration(target) : "–"}
                  </td>
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {session?.check_in_time
                      ? formatTimeOnly(session.check_in_time)
                      : "–"}
                  </td>
                  <td className="px-4 py-3 text-gray-700 tabular-nums">
                    {session?.check_out_time
                      ? formatTimeOnly(session.check_out_time)
                      : session
                        ? "offen"
                        : "–"}
                  </td>
                  <td className="px-4 py-3 text-right text-gray-500 tabular-nums">
                    {session ? formatDuration(session.break_minutes) : "–"}
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

type RowStatus =
  | { kind: "auto-closed" }
  | { kind: "home-office" }
  | { kind: "missing"; future: boolean };

function computeRowStatus(
  session: StaffHistorySession | undefined,
  target: number,
  isFuture: boolean,
): RowStatus | null {
  if (!session) {
    if (target > 0 && !isFuture) {
      return { kind: "missing", future: false };
    }
    return null;
  }
  if (session.auto_checked_out) {
    return { kind: "auto-closed" };
  }
  if (session.status === "home_office") {
    return { kind: "home-office" };
  }
  return null;
}

function StatusBadge({ status }: { readonly status: RowStatus }) {
  if (status.kind === "auto-closed") {
    return (
      <span className="inline-flex items-center rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700">
        Auto-Close
      </span>
    );
  }
  if (status.kind === "home-office") {
    return (
      <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
        Homeoffice
      </span>
    );
  }
  return (
    <span className="inline-flex items-center rounded-full bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-500">
      Nicht erfasst
    </span>
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
    target: number;
  } | null;
  readonly onClose: () => void;
  readonly isAdminView: boolean;
}) {
  if (!selected) return null;
  const { day, session, target } = selected;
  const ist = session?.net_minutes ?? 0;
  const delta = session && target > 0 ? ist - target : 0;
  return (
    <Modal isOpen onClose={onClose} title={formatLongDate(day)}>
      <div className="space-y-3 text-sm">
        <DetailRow
          label="Soll"
          value={target > 0 ? formatDuration(target) : "–"}
        />
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
          label="Ist"
          value={session ? formatDuration(ist) : "–"}
          emphasize
        />
        <DetailRow
          label="Δ zur Soll"
          value={session && target > 0 ? formatSignedDuration(delta) : "–"}
        />
        {session?.status === "home_office" && (
          <DetailRow label="Quelle" value="Homeoffice" />
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
