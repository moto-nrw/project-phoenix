"use client";

// Abgänge view of an applied grade transition (#405).
//
// Graduation is a soft delete: the child vanishes from every staff list and
// every per-student route answers 404 for them, which leaves a school no way to
// act on a retention rule or an erasure request for a departed child. This is
// the one surface that shows those children again and offers the endgültige
// Löschung — deliberately a separate, deliberate second step rather than an
// option on the apply dialog, so `Zurücksetzen` stays usable for the whole
// cohort right up until someone decides otherwise.

import { useCallback, useEffect, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  fetchTransitionHistory,
  purgeGraduatedStudent,
  type GradeTransition,
  type GraduateState,
  type TransitionHistoryEntry,
} from "~/lib/grade-transition-api";

const logger = createLogger({ component: "GraduatesModal" });

const STATE_LABELS: Record<GraduateState, string> = {
  alumnus: "Abgegangen",
  restored: "Zurückgeholt",
  purged: "Endgültig gelöscht",
};

const STATE_COLORS: Record<GraduateState, string> = {
  alumnus: LOCATION_COLORS.SCHOOLYARD,
  restored: LOCATION_COLORS.GROUP_ROOM,
  purged: LOCATION_COLORS.UNKNOWN,
};

function StateBadge({ state }: { readonly state: GraduateState }) {
  return (
    <span
      className="inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
      style={{
        backgroundColor: `${STATE_COLORS[state]}1A`,
        color: STATE_COLORS[state],
      }}
    >
      {STATE_LABELS[state]}
    </span>
  );
}

interface GraduatesModalProps {
  readonly transition: GradeTransition;
  readonly onClose: () => void;
  /** Called after at least one child was deleted, so the list behind refreshes. */
  readonly onPurged: () => void;
  readonly canDelete: boolean;
}

export function GraduatesModal({
  transition,
  onClose,
  onPurged,
  canDelete,
}: GraduatesModalProps) {
  const toast = useToast();
  const [entries, setEntries] = useState<TransitionHistoryEntry[] | null>(null);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [isPurging, setIsPurging] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);

  const load = useCallback(async () => {
    setLoadFailed(false);
    try {
      const history = await fetchTransitionHistory(transition.id);
      // Only Abgänge: a promoted child was never hidden and is edited in the
      // Kindersuche like any other.
      setEntries(history.filter((e) => e.action === "graduated"));
      setSelected(new Set());
    } catch (err) {
      logger.error("graduates_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setEntries([]);
      setLoadFailed(true);
    }
  }, [transition.id]);

  useEffect(() => {
    void load();
  }, [load]);

  const deletable = (entries ?? []).filter((e) => e.studentState === "alumnus");
  const selectedIds = [...selected].filter((id) =>
    deletable.some((e) => e.studentId === id),
  );

  const toggle = (studentId: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(studentId)) {
        next.delete(studentId);
      } else {
        next.add(studentId);
      }
      return next;
    });
  };

  const toggleAll = () => {
    setSelected((prev) =>
      prev.size === deletable.length
        ? new Set()
        : new Set(deletable.map((e) => e.studentId)),
    );
  };

  const handlePurge = async () => {
    setIsPurging(true);
    let deleted = 0;
    const failures: string[] = [];

    // Sequential on purpose: each delete takes the per-tenant class-writes gate
    // plus the child's companion-graph locks, so firing them in parallel would
    // queue them against each other anyway and make a partial failure harder to
    // report. A child that fails (e.g. a Laufgemeinschaft precondition) must not
    // stop the rest.
    for (const studentId of selectedIds) {
      const entry = deletable.find((e) => e.studentId === studentId);
      try {
        await purgeGraduatedStudent(studentId);
        deleted += 1;
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.error("graduate_purge_failed", { error: message });
        failures.push(`${entry?.personName ?? studentId}: ${message}`);
      }
    }

    setIsPurging(false);
    setConfirmOpen(false);

    if (deleted > 0) {
      toast.success(
        deleted === 1
          ? "1 Kind endgültig gelöscht."
          : `${deleted} Kinder endgültig gelöscht.`,
      );
      onPurged();
    }
    if (failures.length > 0) {
      toast.error(
        failures.length === 1
          ? (failures[0] ?? "Löschen fehlgeschlagen.")
          : `${failures.length} Kinder konnten nicht gelöscht werden.`,
      );
    }
    await load();
  };

  return (
    <>
      <Modal
        isOpen
        onClose={onClose}
        title={`Abgänge: Jahrgangswechsel ${transition.academicYear}`}
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
        footer={
          <div className="flex items-center justify-between gap-3">
            <span className="text-sm text-gray-500">
              {selectedIds.length > 0
                ? `${selectedIds.length} ausgewählt`
                : `${deletable.length} von ${entries?.length ?? 0} noch löschbar`}
            </span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={onClose}
              >
                Schließen
              </Button>
              {canDelete && (
                <Button
                  type="button"
                  variant="danger"
                  size="md"
                  disabled={selectedIds.length === 0}
                  onClick={() => setConfirmOpen(true)}
                >
                  Endgültig löschen
                </Button>
              )}
            </div>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Diese Kinder haben die OGS mit diesem Jahrgangswechsel verlassen.
            Sie sind in der App ausgeblendet, ihre Daten aber noch vorhanden.
            Über <span className="font-medium">Zurücksetzen</span> lassen sie
            sich zurückholen, solange sie hier nicht endgültig gelöscht wurden.
          </p>

          {entries === null && <Loading />}

          {loadFailed && (
            <p className="rounded-lg border border-gray-200 p-4 text-sm text-[#FF3130]">
              Abgänge konnten nicht geladen werden. Bitte erneut versuchen.
            </p>
          )}

          {entries !== null && !loadFailed && entries.length === 0 && (
            <p className="rounded-lg border border-gray-200 p-4 text-sm text-gray-500">
              Dieser Jahrgangswechsel hatte keine Abgänge.
            </p>
          )}

          {entries !== null && entries.length > 0 && (
            <div className="overflow-hidden rounded-lg border border-gray-200">
              {canDelete && deletable.length > 0 && (
                <label
                  htmlFor="graduates-select-all"
                  className="flex cursor-pointer items-center gap-3 border-b border-gray-200 bg-gray-50 px-4 py-2.5"
                >
                  <Checkbox
                    id="graduates-select-all"
                    checked={
                      selectedIds.length === deletable.length &&
                      deletable.length > 0
                    }
                    onChange={toggleAll}
                  />
                  <span className="text-sm font-medium text-gray-700">
                    Alle löschbaren auswählen
                  </span>
                </label>
              )}
              <ul>
                {entries.map((entry) => {
                  const isDeletable = entry.studentState === "alumnus";
                  return (
                    <li
                      key={entry.id}
                      className="flex items-center gap-3 border-b border-gray-100 px-4 py-2.5 last:border-b-0"
                    >
                      {canDelete && (
                        <Checkbox
                          checked={selected.has(entry.studentId)}
                          disabled={!isDeletable}
                          onChange={() => toggle(entry.studentId)}
                          aria-label={`${entry.personName} auswählen`}
                        />
                      )}
                      <span className="min-w-0 flex-1 truncate text-sm text-gray-900">
                        {entry.personName}
                      </span>
                      <span className="shrink-0 text-sm text-gray-500">
                        {entry.fromClass}
                      </span>
                      <StateBadge state={entry.studentState} />
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
        </div>
      </Modal>

      {confirmOpen && (
        <ConfirmationModal
          isOpen
          onClose={() => setConfirmOpen(false)}
          onConfirm={() => void handlePurge()}
          title="Endgültig löschen?"
          confirmText="Ja, endgültig löschen"
          cancelText="Abbrechen"
          isConfirmLoading={isPurging}
          isDismissDisabled={isPurging}
        >
          <p>
            {selectedIds.length === 1
              ? "1 Kind wird mit allen Daten unwiderruflich gelöscht."
              : `${selectedIds.length} Kinder werden mit allen Daten unwiderruflich gelöscht.`}{" "}
            <span className="font-medium">Zurücksetzen</span> kann{" "}
            {selectedIds.length === 1 ? "dieses Kind" : "diese Kinder"} danach
            nicht mehr zurückholen.
          </p>
        </ConfirmationModal>
      )}
    </>
  );
}
