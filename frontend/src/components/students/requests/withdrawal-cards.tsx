"use client";

/**
 * Komplett-Abmeldungen (#2434) als Karten der gemeinsamen Anfragenliste:
 * offen mit den zwei Aktionen, in der Historie nur noch als Ergebnis.
 * Unverändert aus der früheren großen Listendatei übernommen (#2267).
 */

import { useState } from "react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { CareExitModal } from "~/components/students/care-exit-modal";
import { StudentDeletionModal } from "~/components/students/student-deletion-modal";
import { RequestReviewCard } from "~/components/students/request-review-card";
import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";

export function OpenWithdrawalCard({
  row,
  grouped = false,
  position,
  total,
  finish,
  remove,
}: Readonly<{
  row: CareWithdrawalCompletion;
  grouped?: boolean;
  position?: number;
  total?: number;
  finish: (row: CareWithdrawalCompletion) => void;
  remove: (row: CareWithdrawalCompletion) => void;
}>) {
  const name = `${row.firstName} ${row.lastName}`.trim();
  const overdue = row.urgency === "overdue";
  const card = (
    <RequestReviewCard
      type="care_withdrawal"
      typeLabel="Abmeldung"
      childName={name}
      grouped={grouped}
      summary={`Keine Betreuungstage ab ${formatDate(row.firstBookinglessDay)}`}
      badge={
        <StatusBadge
          tone={overdue ? "red" : "orange"}
          label={overdue ? "Überfällig" : "Geplant"}
        />
      }
      history={{
        kind: "readonly",
        label: overdue ? "Überfällig" : "Geplant",
        tone: overdue ? "red" : "orange",
      }}
      action={
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            className="max-sm:min-h-11"
            onClick={() => finish(row)}
          >
            Betreuung beenden
          </Button>
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            className="max-sm:min-h-11"
            onClick={() => remove(row)}
          >
            Kind sofort löschen
          </Button>
        </div>
      }
    >
      <p className="text-sm text-gray-600">
        Für dieses Kind ist kein Betreuungstag mehr gebucht. Beenden Sie jetzt
        die Betreuung.
      </p>
    </RequestReviewCard>
  );
  if (!grouped || position === undefined || total === undefined) return card;
  return (
    <div className="border-t border-gray-100 first:border-t-0">
      <div className="flex items-center justify-between gap-2 px-4 pt-3 sm:px-5">
        <p className="text-xs font-medium text-gray-500">
          Anfrage {position} von {total}
        </p>
        <p className="text-xs text-gray-600">Nur einzeln entscheiden</p>
      </div>
      {card}
    </div>
  );
}

export function HistoryWithdrawalCard({
  row,
}: Readonly<{ row: CareWithdrawalCompletion }>) {
  const deleted = row.outcome === "deleted";
  const childName =
    deleted || row.studentId === ""
      ? "Gelöschtes Kind"
      : `${row.firstName} ${row.lastName}`.trim();
  return (
    <RequestReviewCard
      type="care_withdrawal"
      typeLabel="Abmeldung"
      childName={childName}
      summary={deleted ? "Kind sofort gelöscht" : "Betreuung beendet"}
      badge={
        <StatusBadge
          tone="gray"
          label={deleted ? "Gelöscht" : "Abgeschlossen"}
        />
      }
      history={{
        kind: "readonly",
        label: deleted ? "Gelöscht" : "Abgeschlossen",
        tone: "gray",
      }}
    >
      {row.resolvedAt && (
        <p className="text-sm text-gray-600">
          Erledigt am {formatDate(row.resolvedAt)}
        </p>
      )}
    </RequestReviewCard>
  );
}

function CareExitDialog({
  row,
  close,
  finished,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  finished: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <CareExitModal
      isOpen
      studentIds={[row.studentId]}
      completionId={row.id}
      firstBookinglessDay={row.firstBookinglessDay}
      onClose={close}
      onFinished={() => finished(row)}
    />
  );
}

function DeletionDialog({
  row,
  close,
  deleted,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  deleted: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <StudentDeletionModal
      isOpen
      studentId={row.studentId}
      completionId={row.id}
      displayName={`${row.firstName} ${row.lastName}`.trim()}
      onClose={close}
      onDeleted={() => deleted(row)}
    />
  );
}

function DeletionWarningDialog({
  row,
  close,
  confirm,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  confirm: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <ConfirmationModal
      isOpen
      onClose={close}
      onConfirm={() => confirm(row)}
      title="Kind sofort löschen"
      confirmText="Löschen prüfen"
      cancelText="Zurück"
      mobileSheet
    >
      <p className="text-sm text-gray-600">
        Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag
        wird nicht abgewartet.
      </p>
    </ConfirmationModal>
  );
}

/**
 * Die drei Abmelde-Dialoge samt ihrem Zustand an einer Stelle (#2267). Die
 * Anfragenliste braucht davon nur zwei Rückrufe und einen Knoten zum Rendern.
 */
export function useWithdrawalDialogs(
  onFinished: (row: CareWithdrawalCompletion, deleted?: boolean) => void,
) {
  const [careExit, setCareExit] = useState<CareWithdrawalCompletion | null>(
    null,
  );
  const [deletion, setDeletion] = useState<CareWithdrawalCompletion | null>(
    null,
  );
  const [deletionWarning, setDeletionWarning] =
    useState<CareWithdrawalCompletion | null>(null);
  const dialogs = (
    <>
      <CareExitDialog
        row={careExit}
        close={() => setCareExit(null)}
        finished={(row) => {
          setCareExit(null);
          onFinished(row);
        }}
      />
      <DeletionDialog
        row={deletion}
        close={() => setDeletion(null)}
        deleted={(row) => {
          setDeletion(null);
          onFinished(row, true);
        }}
      />
      <DeletionWarningDialog
        row={deletionWarning}
        close={() => setDeletionWarning(null)}
        confirm={(row) => {
          setDeletion(row);
          setDeletionWarning(null);
        }}
      />
    </>
  );
  return {
    dialogs,
    finishWithdrawal: setCareExit,
    removeWithdrawal: setDeletionWarning,
  };
}
