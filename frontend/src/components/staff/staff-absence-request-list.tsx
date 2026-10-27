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

import { useCallback, useEffect, useState } from "react";

import { TrayIcon } from "@phosphor-icons/react/ssr";
import { useSession } from "next-auth/react";
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
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import {
  absenceRowLabel,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
} from "~/lib/absence-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  staffAbsenceService,
  type StaffAbsenceRequestRow,
} from "~/lib/staff-api";
import { useSWRAuth, useTenantUpdateMatching } from "~/lib/swr";

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
  onCountChange,
}: Readonly<{
  view: "open" | "history";
  filters: StaffAbsenceRequestFilters;
  /** Meldet die geladene Zeilenzahl an den Seitenkopf (Statuszeile). */
  onCountChange?: (count: number | null) => void;
}>) {
  const toast = useToast();
  const { mutate: swrMutate } = useSWRConfig();
  const { data: session } = useSession();
  const accountId = session?.user.id;
  const { show: showActionError } = useApiErrorDisplay();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [denyModal, setDenyModal] = useState<StaffAbsenceRequestRow | null>(
    null,
  );
  const [questionModal, setQuestionModal] =
    useState<StaffAbsenceRequestRow | null>(null);

  // Die Anträge liegen im SWR-Cache, je Konto, Ansicht und Filter (#4988):
  // beim erneuten Öffnen steht die Liste sofort, der Abgleich läuft im
  // Hintergrund. Beim Wechsel von Ansicht oder Filter zeigt die Liste ihr
  // Skelett statt der Zeilen des alten Filters; ein Fehlschlag steht mit
  // „Wiederholen“ vor Ort, statt still wiederholt zu werden.
  const listKey = accountId
    ? `staff-absence-requests:${accountId}:${view}:${JSON.stringify([
        filters.search,
        filters.types,
      ])}`
    : null;
  const {
    data: rows,
    error,
    mutate,
  } = useSWRAuth<StaffAbsenceRequestRow[]>(
    listKey,
    () =>
      staffAbsenceService.listRequests(view, {
        search: filters.search,
        types: filters.types,
      }),
    {
      keepPreviousData: false,
      shouldRetryOnError: false,
      // Jedes erneute Öffnen gleicht ab, auch kurz nach dem letzten Abruf.
      dedupingInterval: 0,
    },
  );
  const updateOtherLists = useTenantUpdateMatching([
    `staff-absence-requests:${accountId}:`,
  ]);
  const loadError = useSwrLoadError(error, "die Liste der Anträge", () =>
    mutate(),
  );
  // Die Zeilen, die beim letzten Entscheiden galten. Scheitert das Neuladen
  // danach, wären sie falsch; dann steht der Fehler an ihrer Stelle.
  const [rowsAtDecision, setRowsAtDecision] = useState<
    StaffAbsenceRequestRow[] | undefined
  >(undefined);
  const loading = rows === undefined && error === undefined;
  // Synchron aus dem SWR-Fehler: der Katalogtext lädt nach, bis dahin darf
  // kein Leerzustand („Keine offenen Anträge“) aufblitzen.
  const failed =
    error !== undefined && (rows === undefined || rows === rowsAtDecision);

  useEffect(() => {
    if (error === undefined) return;
    logger.warn("absence_request_list_load_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }, [error]);

  // Solange geladen wird, meldet die Liste `null`: der Seitenkopf zeigt dann
  // einen Skelett-Balken statt einer falschen Null.
  const reportedCount = loading || failed ? null : (rows?.length ?? null);
  useEffect(() => {
    onCountChange?.(reportedCount);
  }, [onCountChange, reportedCount]);

  // Nach einer Entscheidung den Cache bereinigen und neu laden: eine
  // Rückfrage lässt den Antrag offen, genehmigt und abgelehnt verschwinden
  // aus der Arbeitsliste auch dann, wenn der Abgleich fehlschlägt. Dazu die
  // Zähler auffrischen: Badge der Seitenleiste, Verweis auf der Mitarbeiter-
  // Seite und Reiter der Mitarbeitenden-Detailseite hören auf diese Signale.
  const afterDecision = useCallback(
    (decidedId?: string) => {
      setRowsAtDecision(rows);
      if (decidedId) {
        void mutate(
          (current) => current?.filter((row) => row.id !== decidedId),
          { revalidate: true },
        );
      } else {
        void mutate();
      }
      // Ein entschiedener Antrag verschwindet auch aus den anderen Ansichten
      // und Filtern; er käme sonst beim Filterwechsel zurück (#4988). Eine
      // Rückfrage lässt ihn offen.
      if (listKey && decidedId)
        void updateOtherLists(
          (current) =>
            (current as StaffAbsenceRequestRow[] | undefined)?.filter(
              (row) => row.id !== decidedId,
            ),
          [listKey],
        );
      swrMutate(
        (key) =>
          typeof key === "string" && key.includes("staff-pending-absences-"),
      );
      dispatchAbsencesRefresh();
    },
    [listKey, mutate, rows, swrMutate, updateOtherLists],
  );

  const handleApprove = async (row: StaffAbsenceRequestRow) => {
    setBusyId(row.id);
    try {
      await staffAbsenceService.approve(row.id);
      toast.success("Der Antrag ist genehmigt.");
      afterDecision(row.id);
    } catch (err) {
      logger.error("absence_approve_failed", {
        absence_id: row.id,
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: "die Anfrage",
        retry: () => void handleApprove(row),
      });
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

  if (failed || rows === undefined) {
    return <LoadErrorAlert error={loadError} />;
  }

  // Ein gescheiterter Abgleich im Hintergrund: die Liste bleibt stehen, der
  // Fehler steht mit „Wiederholen“ darüber.
  const revalidationError = <LoadErrorAlert error={loadError} />;

  if (rows.length === 0) {
    return (
      <>
        {revalidationError}
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
      </>
    );
  }

  if (view === "history") {
    return (
      <>
        {revalidationError}
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
      </>
    );
  }

  return (
    <>
      {revalidationError}
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
              afterDecision(denyModal.id);
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
    </>
  );
}
