"use client";

import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { useSWRConfig } from "swr";

import {
  DenyAbsenceModal,
  QuestionAbsenceModal,
} from "~/components/staff/absence-decision-modals";
import { Avatar } from "~/components/ui/avatar";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import {
  ABSENCE_TYPE_COLOR,
  ABSENCE_TYPE_LABEL,
  absenceStatusMeta,
  dayCountFor,
  dispatchAbsencesRefresh,
  formatAbsenceDate,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import {
  staffAbsenceService,
  type Staff,
  type StaffAbsenceRow,
} from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "StaffPendingInbox" });

// SWR key for the tenant-wide open-request list. Deliberately shares the
// "staff-pending-absences-" prefix with the per-staff detail-tab badge so the
// existing includes()-mutate in abwesenheiten-tab.tsx invalidates both.
export const STAFF_PENDING_ABSENCES_KEY = "staff-pending-absences-all";

// Fetches the tenant-wide open absence requests for the /staff inbox (#1419).
// Gated on vacation:approve (matching the backend endpoint) so non-approvers
// never fire the request.
export function useStaffPendingInbox() {
  const { data: session } = useSession();
  const canReview =
    isAdmin(session) || hasPermission(session, "vacation:approve");
  const { data, mutate } = useSWRAuth<StaffAbsenceRow[]>(
    canReview ? STAFF_PENDING_ABSENCES_KEY : null,
    () => staffAbsenceService.listPending(),
    { revalidateOnFocus: true },
  );
  return { rows: data ?? [], canReview, refresh: mutate };
}

const TYPE_TABS = [
  { value: "all", label: "Alle" },
  { value: "vacation", label: "Urlaub" },
  { value: "sick", label: "Krank" },
  { value: "training", label: "Fortbildung" },
  { value: "other", label: "Sonstige" },
] as const;

export function StaffPendingInbox({
  rows,
  staffList,
  onRefresh,
}: {
  readonly rows: StaffAbsenceRow[];
  readonly staffList: Staff[];
  readonly onRefresh: () => void;
}) {
  const toast = useToast();
  const { mutate: swrMutate } = useSWRConfig();
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [busyId, setBusyId] = useState<number | null>(null);
  const [denyModal, setDenyModal] = useState<StaffAbsenceRow | null>(null);
  const [questionModal, setQuestionModal] = useState<StaffAbsenceRow | null>(
    null,
  );

  const staffById = useMemo(() => {
    const map = new Map<number, Staff>();
    for (const s of staffList) {
      map.set(Number(s.id), s);
    }
    return map;
  }, [staffList]);

  const filtered = useMemo(
    () =>
      typeFilter === "all"
        ? rows
        : rows.filter((row) => row.absence_type === typeFilter),
    [rows, typeFilter],
  );

  const afterMutation = () => {
    onRefresh();
    // Per-staff detail-tab badges share the key prefix; the sidebar counter
    // listens for the refresh event.
    swrMutate(
      (key) =>
        typeof key === "string" && key.includes("staff-pending-absences-"),
    );
    dispatchAbsencesRefresh();
  };

  const handleApprove = async (row: StaffAbsenceRow) => {
    setBusyId(row.id);
    try {
      await staffAbsenceService.approve(row.id);
      toast.success("Antrag genehmigt.");
      afterMutation();
    } catch (err) {
      logger.error("approve_failed", {
        absence_id: row.id,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        err instanceof Error ? err.message : "Genehmigung fehlgeschlagen.",
      );
    } finally {
      setBusyId(null);
    }
  };

  if (rows.length === 0) {
    return (
      <div className="mb-6 flex items-center gap-3 rounded-2xl border border-gray-200 bg-white/60 px-5 py-3 text-sm text-gray-500">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-[#83CD2D]/15 text-[#4a7a15]">
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </span>
        <p className="text-sm font-medium text-gray-600">
          Keine offenen Anfragen
        </p>
      </div>
    );
  }

  return (
    <div className="mb-6 rounded-2xl border-2 border-amber-200 bg-amber-50/40 p-5 shadow-sm">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-amber-500 text-[10px] font-bold text-white">
            {rows.length}
          </span>
          <h2 className="text-sm font-bold tracking-wide text-amber-900 uppercase">
            Eingehende Anfragen
          </h2>
        </div>
        <Tabs value={typeFilter} onValueChange={setTypeFilter}>
          <TabsList>
            {TYPE_TABS.map((tab) => (
              <TabsTrigger key={tab.value} value={tab.value}>
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      {filtered.length === 0 ? (
        <p className="py-4 text-center text-sm text-gray-500">
          Keine offenen Anfragen für diesen Typ.
        </p>
      ) : (
        <ul className="space-y-2">
          {filtered.map((row) => {
            const staffMember = staffById.get(row.staff_id);
            const staffName = staffMember
              ? `${staffMember.firstName} ${staffMember.lastName}`
              : `Mitarbeiter:in #${row.staff_id}`;
            const isQuestioned = row.status === "question";
            const isBusy = busyId === row.id;
            return (
              <li
                key={row.id}
                className="rounded-2xl border border-amber-100 bg-white p-4"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="flex min-w-0 flex-1 items-start gap-3">
                    <Avatar name={staffName} size="md" />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-bold text-gray-900">
                          {staffName}
                        </p>
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${ABSENCE_TYPE_COLOR[row.absence_type] ?? "bg-gray-100 text-gray-600"}`}
                        >
                          {ABSENCE_TYPE_LABEL[row.absence_type] ??
                            row.absence_type}
                        </span>
                        {isQuestioned && (
                          <span
                            className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${absenceStatusMeta(row.status).className}`}
                          >
                            {absenceStatusMeta(row.status).label}
                          </span>
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
                      {isQuestioned && row.decision_note && (
                        <p className="mt-1 text-xs text-purple-700">
                          <span className="font-medium">Rückfrage:</span>{" "}
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
                  <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row">
                    {!isQuestioned && (
                      <button
                        type="button"
                        onClick={() => setQuestionModal(row)}
                        disabled={isBusy}
                        className="rounded-lg border border-[#7C3AED]/40 px-3 py-1.5 text-xs font-medium text-[#7C3AED] transition-colors hover:bg-[#7C3AED]/5 disabled:opacity-50"
                      >
                        Rückfrage
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => setDenyModal(row)}
                      disabled={isBusy}
                      className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
                    >
                      Ablehnen
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        handleApprove(row);
                      }}
                      disabled={isBusy}
                      className="rounded-lg bg-[#83CD2D] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-[#74b827] disabled:opacity-50"
                    >
                      {isBusy ? "…" : "Genehmigen"}
                    </button>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {denyModal && (
        <DenyAbsenceModal
          absence={denyModal}
          onClose={() => setDenyModal(null)}
          onDenied={() => {
            setDenyModal(null);
            afterMutation();
          }}
        />
      )}
      {questionModal && (
        <QuestionAbsenceModal
          absence={questionModal}
          onClose={() => setQuestionModal(null)}
          onQuestioned={() => {
            setQuestionModal(null);
            afterMutation();
          }}
        />
      )}
    </div>
  );
}
