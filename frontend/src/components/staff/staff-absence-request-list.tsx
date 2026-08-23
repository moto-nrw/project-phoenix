"use client";

/**
 * Mitarbeitende-Reiter des Anfragen-Moduls (#2433): Abwesenheitsanträge
 * (Urlaub, Krank, Fortbildung, Sonstige) offen entscheiden oder in der
 * Historie nachschlagen. Suche nach Namen und Art-Filter wirken serverseitig.
 *
 * Die offenen Anträge behalten die Zeilen und Modals der bisherigen Inbox von
 * der Mitarbeiter-Seite; die Historie nutzt dieselbe Karte wie der
 * Eltern-Reiter, damit beide Reiter gleich aussehen.
 */

import { useCallback, useEffect, useRef, useState } from "react";

import { TrayIcon } from "@phosphor-icons/react/ssr";
import { useSWRConfig } from "swr";

import {
  DenyAbsenceModal,
  QuestionAbsenceModal,
} from "~/components/staff/absence-decision-modals";
import { AbsenceRequestRow } from "~/components/staff/absence-request-row";
import {
  RequestReviewCard,
  RequestRowHeader,
} from "~/components/students/request-review-card";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useToast } from "~/contexts/ToastContext";
import {
  absenceRowLabel,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
} from "~/lib/absence-helpers";
import { createLogger } from "~/lib/logger";
import {
  staffAbsenceService,
  type StaffAbsenceRequestRow,
} from "~/lib/staff-api";

const logger = createLogger({ component: "StaffAbsenceRequestList" });

export interface StaffAbsenceRequestFilters {
  readonly search: string;
  /** Leer = alle Arten. */
  readonly types: readonly string[];
}

// Die Historie-Karte kennt drei Status; die Abwesenheiten haben eigene Namen
// dafür. „canceled" heißt hier zurückgezogen: der MA hat den Antrag selbst
// zurückgenommen.
const HISTORY_STATUS: Record<string, string> = {
  approved: "approved",
  declined: "rejected",
  canceled: "withdrawn",
};

function staffLabel(row: StaffAbsenceRequestRow): string {
  return row.staff_name.trim() || "Unbekannt";
}

// Nur entschiedene Anträge haben eine entscheidende Person. Ist das Konto
// inzwischen gelöscht, steht dort "Unbekannt"; ein zurückgezogener Antrag
// bekommt gar keinen Namen.
function decidedByLabel(row: StaffAbsenceRequestRow): string | undefined {
  if (row.status === "canceled") return undefined;
  if (!row.approved_by) return undefined;
  return row.decided_by_name?.trim() || "Unbekannt";
}

export function StaffAbsenceRequestList({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: StaffAbsenceRequestFilters;
}>) {
  const toast = useToast();
  const { mutate: swrMutate } = useSWRConfig();
  const [rows, setRows] = useState<StaffAbsenceRequestRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const loadVersion = useRef(0);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [denyModal, setDenyModal] = useState<StaffAbsenceRequestRow | null>(
    null,
  );
  const [questionModal, setQuestionModal] =
    useState<StaffAbsenceRequestRow | null>(null);

  const load = useCallback(
    () =>
      staffAbsenceService.listRequests(view, {
        search: filters.search,
        types: filters.types,
      }),
    [view, filters],
  );

  useEffect(() => {
    let cancelled = false;
    const version = ++loadVersion.current;
    setLoading(true);
    setError(null);
    load()
      .then((data) => {
        if (!cancelled && version === loadVersion.current) setRows(data);
      })
      .catch((err: unknown) => {
        if (cancelled || version !== loadVersion.current) return;
        logger.warn("absence_request_list_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Anträge konnten nicht geladen werden.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [load]);

  // Nach einer Entscheidung neu laden statt die Zeile zu entfernen: eine
  // Rückfrage lässt den Antrag offen, genehmigt und abgelehnt verschwinden
  // aus der Arbeitsliste. Dazu die Zähler auffrischen — Badge der
  // Seitenleiste, Verweis auf der Mitarbeiter-Seite, Reiter der
  // Mitarbeitenden-Detailseite hören auf diese beiden Signale.
  const afterDecision = useCallback(() => {
    const version = ++loadVersion.current;
    load()
      .then((data) => {
        if (version === loadVersion.current) setRows(data);
      })
      .catch((err: unknown) => {
        logger.warn("absence_request_list_reload_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    swrMutate(
      (key) =>
        typeof key === "string" && key.includes("staff-pending-absences-"),
    );
    dispatchAbsencesRefresh();
  }, [load, swrMutate]);

  const handleApprove = async (row: StaffAbsenceRequestRow) => {
    setBusyId(row.id);
    try {
      await staffAbsenceService.approve(row.id);
      toast.success("Antrag genehmigt.");
      afterDecision();
    } catch (err) {
      logger.error("absence_approve_failed", {
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

  if (loading) {
    return (
      <SkeletonRegion label="Anträge werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  const hasFilters = filters.search.trim() !== "" || filters.types.length > 0;

  if (error) {
    return <Alert type="error" message={error} />;
  }

  if (rows.length === 0) {
    return (
      <EmptyState
        icon={<TrayIcon size={32} aria-hidden="true" />}
        title={
          view === "open"
            ? "Keine offenen Anträge."
            : "Noch keine entschiedenen Anträge."
        }
        description={
          hasFilters
            ? "Für die aktuelle Suche und Filter gibt es keine Treffer."
            : undefined
        }
        variant="compact"
      />
    );
  }

  if (view === "history") {
    return (
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <RequestRowHeader view="history" />
        {rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            type="absence"
            typeLabel={absenceRowLabel(row)}
            childName={staffLabel(row)}
            summary={formatAbsenceRange(row.date_start, row.date_end)}
            submittedAt={row.requested_at}
            history={{
              status: HISTORY_STATUS[row.status] ?? row.status,
              decidedAt:
                row.status === "canceled"
                  ? (row.updated_at ?? "")
                  : (row.approved_at ?? ""),
              decidedByName: decidedByLabel(row),
              reason: row.decision_note,
            }}
          >
            {row.note && (
              <p className="text-sm text-gray-700">
                <span className="font-medium">Notiz:</span> {row.note}
              </p>
            )}
          </RequestReviewCard>
        ))}
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-5">
      <ul className="divide-y divide-gray-100">
        {rows.map((row) => (
          <AbsenceRequestRow
            key={row.id}
            row={row}
            staffName={staffLabel(row)}
            isBusy={busyId === row.id}
            showActions
            onApprove={() => {
              void handleApprove(row);
            }}
            onDeny={() => setDenyModal(row)}
            onQuestion={() => setQuestionModal(row)}
          />
        ))}
      </ul>

      {denyModal && (
        <DenyAbsenceModal
          absence={denyModal}
          onClose={() => setDenyModal(null)}
          onDenied={() => {
            afterDecision();
            setDenyModal(null);
          }}
        />
      )}
      {questionModal && (
        <QuestionAbsenceModal
          absence={questionModal}
          onClose={() => setQuestionModal(null)}
          onQuestioned={() => {
            afterDecision();
            setQuestionModal(null);
          }}
        />
      )}
    </div>
  );
}
