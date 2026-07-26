"use client";

import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { useSWRConfig } from "swr";

import {
  DenyAbsenceModal,
  QuestionAbsenceModal,
} from "~/components/staff/absence-decision-modals";
import { AbsenceRequestRow } from "~/components/staff/absence-request-row";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import { dispatchAbsencesRefresh } from "~/lib/absence-helpers";
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
const STAFF_PENDING_ABSENCES_KEY = "staff-pending-absences-all";

// Fetches the tenant-wide open absence requests for the /staff inbox (#1419).
// Gated on vacation:approve (matching the backend endpoint) so non-approvers
// never fire the request.
export function useStaffPendingInbox() {
  const { data: session } = useSession();
  const canReview =
    isAdmin(session) || hasPermission(session, "vacation:approve");
  const { data } = useSWRAuth<StaffAbsenceRow[]>(
    canReview ? STAFF_PENDING_ABSENCES_KEY : null,
    () => staffAbsenceService.listPending(),
    { revalidateOnFocus: true },
  );
  return { rows: data ?? [], canReview };
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
}: {
  readonly rows: StaffAbsenceRow[];
  readonly staffList: Staff[];
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
    // The tenant-wide inbox and per-staff detail-tab badges share the key
    // prefix; one predicate refreshes each matching key exactly once. The
    // sidebar counter listens for the separate refresh event.
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

  // Nothing pending → no panel at all; the sidebar counter already covers
  // the "alles bearbeitet" case and the list page stays clean.
  if (rows.length === 0) {
    return null;
  }

  return (
    <div className="moto-content-surface mb-6 rounded-2xl border p-4 shadow-sm sm:p-5">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h2 className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
            Eingehende Anfragen
          </h2>
          <span className="inline-flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white">
            {rows.length}
          </span>
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
        <ul className="divide-y divide-gray-100">
          {filtered.map((row) => {
            const staffMember = staffById.get(row.staff_id);
            const staffName = staffMember
              ? `${staffMember.firstName} ${staffMember.lastName}`
              : `Mitarbeiter:in #${row.staff_id}`;
            return (
              <AbsenceRequestRow
                key={row.id}
                row={row}
                staffName={staffName}
                isBusy={busyId === row.id}
                showActions
                onApprove={(r) => {
                  handleApprove(r);
                }}
                onDeny={setDenyModal}
                onQuestion={setQuestionModal}
              />
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
