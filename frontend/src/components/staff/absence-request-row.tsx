"use client";

import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  ABSENCE_TYPE_HEX,
  ABSENCE_TYPE_LABEL,
  absenceStatusMeta,
  dayCountFor,
  formatAbsenceDate,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { StaffAbsenceRequestRow, StaffAbsenceRow } from "~/lib/staff-api";

type AbsenceRequestRowData = Omit<
  StaffAbsenceRow,
  "id" | "staff_id" | "approved_by"
> & {
  id: string | number;
};

// One pending-request row shared by the /staff inbox and the per-staff
// Abwesenheiten tab (#1419). Rendered inside a `divide-y divide-gray-100`
// list. `staffName` adds the avatar + name block (tenant-wide inbox); the
// per-staff tab omits it.
export function AbsenceRequestRow<
  T extends AbsenceRequestRowData | StaffAbsenceRequestRow,
>({
  row,
  staffName,
  isBusy,
  showActions,
  onApprove,
  onDeny,
  onQuestion,
}: {
  readonly row: T;
  readonly staffName?: string;
  readonly isBusy: boolean;
  readonly showActions: boolean;
  readonly onApprove: (row: T) => void;
  readonly onDeny: (row: T) => void;
  readonly onQuestion: (row: T) => void;
}) {
  const isQuestioned = row.status === "question";
  const statusMeta = absenceStatusMeta(row.status);
  return (
    <li className="py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1 items-start gap-3">
          {staffName && <Avatar name={staffName} size="md" />}
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              {staffName && (
                <p className="text-sm font-bold text-gray-900">{staffName}</p>
              )}
              <StatusDotBadge
                label={ABSENCE_TYPE_LABEL[row.absence_type] ?? row.absence_type}
                color={
                  ABSENCE_TYPE_HEX[row.absence_type] ?? LOCATION_COLORS.UNKNOWN
                }
              />
              {isQuestioned && (
                <StatusDotBadge
                  label={statusMeta.label}
                  color={statusMeta.color}
                />
              )}
            </div>
            <p className="mt-0.5 text-sm text-gray-700">
              {formatAbsenceRange(row.date_start, row.date_end)}
              <span className="ml-2 text-xs text-gray-500">
                ·{" "}
                {formatDayCount(
                  dayCountFor({
                    workingDays: row.working_days,
                    dateStart: row.date_start,
                    dateEnd: row.date_end,
                    halfDay: row.half_day,
                    startHalfDay: row.start_half_day,
                    endHalfDay: row.end_half_day,
                    hasBoundaryFields:
                      row.start_half_day !== undefined ||
                      row.end_half_day !== undefined,
                  }),
                )}
              </span>
            </p>
            {row.note && (
              <p className="mt-1 text-xs text-gray-600">
                <span className="font-medium">Notiz:</span> {row.note}
              </p>
            )}
            {row.decision_note &&
              (isQuestioned || row.status === "requested") && (
                <p className="mt-1 text-xs text-gray-600">
                  <span className="font-medium">
                    {isQuestioned ? "Rückfrage:" : "Vorherige Rückfrage:"}
                  </span>{" "}
                  {row.decision_note}
                </p>
              )}
            {row.requested_at && (
              <p className="mt-1 text-[11px] text-gray-400">
                Eingegangen {formatAbsenceDate(row.requested_at)}
              </p>
            )}
          </div>
        </div>
        {showActions && (
          <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row">
            {!isQuestioned && (
              <Button
                type="button"
                variant="outline"
                size="compact"
                onClick={() => onQuestion(row)}
                disabled={isBusy}
              >
                Rückfrage
              </Button>
            )}
            <Button
              type="button"
              variant="outline"
              size="compact"
              onClick={() => onDeny(row)}
              disabled={isBusy}
            >
              Ablehnen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="compact"
              onClick={() => onApprove(row)}
              disabled={isBusy}
            >
              {isBusy ? "…" : "Genehmigen"}
            </Button>
          </div>
        )}
      </div>
    </li>
  );
}
