"use client";

// Jahrgangsstufenwechsel admin flow (#405): list of transitions, draft
// editor, preview + apply, revert. Consumes /api/admin/grade-transitions.

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  deleteGradeTransition,
  listGradeTransitions,
  NOT_APPLIED_CODE,
  NOT_DRAFT_CODE,
  NOT_LATEST_TRANSITION_CODE,
  revertGradeTransition,
  TransitionRequestError,
  type GradeTransition,
  type TransitionResult,
  type TransitionStatus,
} from "~/lib/grade-transition-api";
import { GraduatesModal } from "./graduates-modal";
import { TransitionEditor } from "./transition-editor";
import { TransitionPreviewModal } from "./transition-preview-modal";

const logger = createLogger({ component: "GradeTransitionsManager" });

const STATUS_LABELS: Record<TransitionStatus, string> = {
  draft: "Entwurf",
  applied: "Angewendet",
  reverted: "Zurückgesetzt",
};

const STATUS_COLORS: Record<TransitionStatus, string> = {
  draft: LOCATION_COLORS.UNKNOWN,
  applied: LOCATION_COLORS.GROUP_ROOM,
  reverted: LOCATION_COLORS.SCHOOLYARD,
};

function StatusBadge({ status }: { readonly status: TransitionStatus }) {
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium"
      style={{
        backgroundColor: `${STATUS_COLORS[status]}1A`,
        color: STATUS_COLORS[status],
      }}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}

/**
 * Per-action gates for the transition controls. Each maps to the matching
 * backend permission, so a user only ever sees actions the backend will
 * actually allow. Defaults to full access when the caller does not resolve
 * permissions (e.g. isolated tests).
 */
export interface TransitionPermissions {
  readonly canCreate: boolean;
  readonly canUpdate: boolean;
  readonly canDelete: boolean;
  readonly canApply: boolean;
  readonly canPurge: boolean;
}

const FULL_ACCESS: TransitionPermissions = {
  canCreate: true,
  canUpdate: true,
  canDelete: true,
  canApply: true,
  canPurge: true,
};

function describeMappings(transition: GradeTransition): string {
  if (transition.mappings.length === 0) return "Keine Zuordnungen";
  const promotions = transition.mappings.filter((m) => m.toClass !== null);
  const graduations = transition.mappings.length - promotions.length;
  const parts: string[] = [];
  if (promotions.length > 0) {
    parts.push(
      `${promotions.length} ${promotions.length === 1 ? "Klasse" : "Klassen"} versetzen`,
    );
  }
  if (graduations > 0) {
    parts.push(`${graduations} Abgang`);
  }
  return parts.join(", ");
}

// Only the most-recently-applied transition may be reverted. Reverting an older
// one writes each student's recorded from_class back, clobbering the class a
// later transition (or a manual edit) has since assigned. Gating the action to
// the latest applied row enforces a strict reverse-order unwind: once it is
// reverted the previous transition becomes the latest and can be reverted in
// turn. The backend enforces the same order and answers a stale target with a
// 409 (NOT_LATEST_TRANSITION_CODE), so this runs over a freshly fetched list too.
//
// The comparison must reproduce the backend's ordering EXACTLY
// (`applied_at DESC NULLS LAST, id DESC` in LockLatestApplied). Picking the
// wrong one of two close applies means the UI offers a target the backend
// rejects with 409, refetches, and picks the same invalid target again: an
// unbreakable loop for the admin (#405 review).
//
// Two properties of the wire format make the string compare below faithful:
// applied_at is serialized in UTC with fixed-width microsecond precision (see
// timeFormatISO8601 in api/admin/grade_transitions.go), so it is both as precise
// as the database column and lexically ordered. The id tiebreak below therefore
// only decides genuinely equal timestamps — legacy rows applied before the
// column carried one.
// Exact ordering for the backend's int64 ids, which cross the wire as decimal
// strings. `Number()` would round anything past 2^53, silently making two
// distinct transitions compare equal and re-opening the 409 loop described
// above; a plain lexical compare would order "9" above "10". For non-negative
// digit strings, longer means larger and equal lengths compare lexically —
// exact over the whole int64 range and, unlike `BigInt()`, unable to throw on a
// malformed id mid-render. Anything not digits-only falls back to a numeric
// compare rather than crashing the manager.
function compareTransitionIDs(a: string, b: string): number {
  if (/^\d+$/.test(a) && /^\d+$/.test(b)) {
    const trimmedA = a.replace(/^0+(?=\d)/, "");
    const trimmedB = b.replace(/^0+(?=\d)/, "");
    if (trimmedA.length !== trimmedB.length) {
      return trimmedA.length - trimmedB.length;
    }
    return trimmedA < trimmedB ? -1 : trimmedA > trimmedB ? 1 : 0;
  }
  return Number(a) - Number(b);
}

function isMoreRecentlyApplied(
  candidate: GradeTransition,
  current: GradeTransition,
): boolean {
  // NULLS LAST: a row with a timestamp always beats one without.
  const a = candidate.appliedAt ?? "";
  const b = current.appliedAt ?? "";
  if (a !== b) return a > b;
  // Tie (identical timestamps, or both untimestamped) — id DESC.
  return compareTransitionIDs(candidate.id, current.id) > 0;
}

function pickLatestRevertable(
  list: readonly GradeTransition[],
): GradeTransition | null {
  let latest: GradeTransition | null = null;
  for (const t of list) {
    if (!t.canRevert || t.status !== "applied") continue;
    if (latest === null || isMoreRecentlyApplied(t, latest)) {
      latest = t;
    }
  }
  return latest;
}

// A revert can only be partial: a promoted child deleted or moved to another
// class since the transition is left alone, and so is a graduate whose status
// changed after the apply. The backend reports each skipped group as a warning —
// swallowing them tells the admin every child was restored when some were not.
//
// The warnings arrive in English (they double as server log text), so the two
// shapes the revert can produce are translated here and anything else is shown
// verbatim rather than dropped.
function describeRevertWarning(warning: string): string {
  const count = /^(\d+)/.exec(warning)?.[1] ?? "";
  if (warning.includes("could not be reverted")) {
    return `${count} versetzte Kinder konnten nicht zurückversetzt werden (seither gelöscht oder in eine andere Klasse verschoben).`.trim();
  }
  if (warning.includes("could not be restored")) {
    return `${count} Abgänge konnten nicht wiederhergestellt werden (seither gelöscht oder Status geändert).`.trim();
  }
  return warning;
}

export function GradeTransitionsManager({
  permissions = FULL_ACCESS,
}: {
  readonly permissions?: TransitionPermissions;
}) {
  const toast = useToast();
  const [transitions, setTransitions] = useState<GradeTransition[] | null>(
    null,
  );
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editorDraft, setEditorDraft] = useState<GradeTransition | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [previewFor, setPreviewFor] = useState<GradeTransition | null>(null);
  const [revertTarget, setRevertTarget] = useState<GradeTransition | null>(
    null,
  );
  const [deleteTarget, setDeleteTarget] = useState<GradeTransition | null>(
    null,
  );
  const [graduatesFor, setGraduatesFor] = useState<GradeTransition | null>(
    null,
  );
  const [busy, setBusy] = useState(false);

  // Returns the freshly fetched list (null when the fetch failed) so callers
  // that must act on the new server state — the stale-revert conflict path —
  // can read it without waiting for the state update to land.
  const refresh = useCallback(async (): Promise<GradeTransition[] | null> => {
    try {
      const list = await listGradeTransitions();
      setTransitions(list);
      setLoadError(null);
      return list;
    } catch (error) {
      logger.error("transitions_load_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setLoadError("Jahrgangswechsel konnten nicht geladen werden.");
      return null;
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleApplied = (result: TransitionResult) => {
    setPreviewFor(null);
    setEditorDraft(null);
    toast.success(
      `Jahrgangswechsel angewendet: ${result.studentsPromoted} Kinder versetzt, ${result.studentsGraduated} Abgänge.`,
    );
    void refresh();
  };

  const handleRevert = async () => {
    if (!revertTarget) return;
    setBusy(true);
    try {
      const result = await revertGradeTransition(revertTarget.id);
      const summary = `${result.studentsPromoted} Kinder zurückversetzt, ${result.studentsGraduated} Abgänge wiederhergestellt`;
      if (result.warnings.length > 0) {
        toast.warning(
          `Jahrgangswechsel nur teilweise zurückgesetzt: ${summary}. ${result.warnings
            .map(describeRevertWarning)
            .join(" ")}`,
          { duration: 12000 },
        );
      } else {
        toast.success(`Jahrgangswechsel zurückgesetzt: ${summary}.`);
      }
      setRevertTarget(null);
      void refresh();
    } catch (error) {
      logger.error("revert_failed", {
        transition_id: revertTarget.id,
        error: error instanceof Error ? error.message : String(error),
      });
      // A stale target is recoverable, but only after reloading: another admin
      // applied a newer transition since this list was fetched, and reverts must
      // unwind newest-first. Retrying the same ID is guaranteed to 409 again, so
      // reload and re-point the dialog at the new latest instead of telling the
      // user to try again against a target that can never succeed (#405 review).
      if (
        error instanceof TransitionRequestError &&
        error.code === NOT_LATEST_TRANSITION_CODE
      ) {
        const list = await refresh();
        const latest = list ? pickLatestRevertable(list) : null;
        setRevertTarget(latest);
        toast.error(
          latest
            ? `Inzwischen wurde ein neuerer Jahrgangswechsel angewendet. Es muss zuerst ${latest.academicYear} zurückgesetzt werden - die Auswahl wurde entsprechend angepasst.`
            : "Inzwischen wurde ein neuerer Jahrgangswechsel angewendet. Die Liste wurde neu geladen.",
        );
      } else if (
        error instanceof TransitionRequestError &&
        error.code === NOT_APPLIED_CODE
      ) {
        // The target is no longer in applied status: another admin already
        // reverted it since this list was fetched. Retrying the same ID can
        // never succeed, so reload the list and close the dialog instead of
        // suggesting a retry (#405 review).
        setRevertTarget(null);
        void refresh();
        toast.error(
          "Der Jahrgangswechsel wurde inzwischen bereits zurückgesetzt. Die Liste wurde neu geladen.",
        );
      } else {
        toast.error("Zurücksetzen fehlgeschlagen. Bitte erneut versuchen.");
      }
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setBusy(true);
    try {
      await deleteGradeTransition(deleteTarget.id);
      toast.success("Entwurf gelöscht.");
      setDeleteTarget(null);
      void refresh();
    } catch (error) {
      logger.error("delete_failed", {
        transition_id: deleteTarget.id,
        error: error instanceof Error ? error.message : String(error),
      });
      // A not_draft conflict means another admin applied this draft since the
      // list was fetched. Retrying the delete can never succeed, so reload the
      // list and close the dialog instead of suggesting a retry (#405 review).
      if (
        error instanceof TransitionRequestError &&
        error.code === NOT_DRAFT_CODE
      ) {
        setDeleteTarget(null);
        void refresh();
        toast.error(
          "Der Entwurf wurde inzwischen angewendet und kann nicht mehr gelöscht werden. Die Liste wurde neu geladen.",
        );
      } else {
        toast.error("Löschen fehlgeschlagen. Bitte erneut versuchen.");
      }
    } finally {
      setBusy(false);
    }
  };

  const openEditorFor = useCallback((transition: GradeTransition | null) => {
    setEditorDraft(transition);
    setEditorOpen(true);
  }, []);

  // See pickLatestRevertable: only the newest applied transition is revertable.
  const latestRevertableId = useMemo(
    () =>
      transitions ? (pickLatestRevertable(transitions)?.id ?? null) : null,
    [transitions],
  );

  const columns = useMemo<DataTableColumn<GradeTransition>[]>(
    () => [
      {
        key: "academicYear",
        header: "Schuljahr",
        render: (t) => (
          <span className="font-medium text-gray-900">{t.academicYear}</span>
        ),
        sortValue: (t) => t.academicYear,
      },
      {
        key: "status",
        header: "Status",
        render: (t) => <StatusBadge status={t.status} />,
      },
      {
        key: "mappings",
        header: "Zuordnungen",
        render: (t) => (
          <span className="text-gray-600">{describeMappings(t)}</span>
        ),
      },
      {
        key: "createdAt",
        header: "Erstellt",
        render: (t) => (
          <span className="text-gray-600">{formatDate(t.createdAt)}</span>
        ),
        sortValue: (t) => t.createdAt,
      },
      {
        key: "appliedAt",
        header: "Angewendet am",
        render: (t) => (
          <span className="text-gray-600">
            {t.appliedAt ? formatDate(t.appliedAt) : "-"}
          </span>
        ),
      },
      {
        key: "actions",
        header: "",
        align: "right",
        render: (t) => (
          <div className="flex justify-end gap-1">
            {t.canModify && permissions.canUpdate && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => openEditorFor(t)}
              >
                Bearbeiten
              </Button>
            )}
            {t.canApply && permissions.canApply && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setPreviewFor(t)}
              >
                Anwenden
              </Button>
            )}
            {t.canModify && permissions.canDelete && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="text-moto-red"
                onClick={() => setDeleteTarget(t)}
              >
                Löschen
              </Button>
            )}
            {t.id === latestRevertableId && permissions.canApply && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setRevertTarget(t)}
              >
                Zurücksetzen
              </Button>
            )}
            {/* Every transition that ran has Abgänge worth inspecting, including
                a reverted one — a child hard-deleted before the revert stays
                gone, and this is the only place that says so. */}
            {t.status !== "draft" && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setGraduatesFor(t)}
              >
                Abgänge
              </Button>
            )}
          </div>
        ),
      },
    ],
    [openEditorFor, permissions, latestRevertableId],
  );

  return (
    <div className="space-y-4">
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">
              Jahrgangswechsel
            </h2>
            <p className="text-sm text-gray-600">
              Versetzt alle Kinder in die nächste Klasse und verwaltet Abgänge
              zum Schuljahreswechsel.
            </p>
          </div>
          {permissions.canCreate && (
            <Button type="button" size="md" onClick={() => openEditorFor(null)}>
              Neuer Jahrgangswechsel
            </Button>
          )}
        </div>
      </div>

      {loadError && <p className="text-moto-red text-sm">{loadError}</p>}

      <DataTable
        columns={columns}
        rows={transitions ?? []}
        getRowKey={(t: GradeTransition) => t.id}
        isLoading={transitions === null && !loadError}
        defaultSortKey="createdAt"
        defaultSortDirection="desc"
        emptyState={
          <div className="py-8 text-center text-sm text-gray-500">
            <p className="font-medium text-gray-700">
              Noch kein Jahrgangswechsel angelegt.
            </p>
            <p className="mt-1">
              Mit Neuer Jahrgangswechsel werden alle Klassen automatisch
              vorgeschlagen (z. B. 1a in 2a) und lassen sich vor dem Anwenden
              anpassen.
            </p>
          </div>
        }
      />

      {editorOpen && (
        <TransitionEditor
          existingDraft={editorDraft}
          onClose={() => {
            setEditorOpen(false);
            setEditorDraft(null);
            void refresh();
          }}
          onReadyForPreview={(transition) => {
            setEditorOpen(false);
            setEditorDraft(null);
            setPreviewFor(transition);
          }}
        />
      )}

      {previewFor && (
        <TransitionPreviewModal
          transition={previewFor}
          canApply={permissions.canApply}
          canEdit={permissions.canUpdate}
          onClose={() => {
            setPreviewFor(null);
            void refresh();
          }}
          onBackToEditor={() => {
            setPreviewFor(null);
            openEditorFor(previewFor);
          }}
          onApplied={handleApplied}
        />
      )}

      {graduatesFor && (
        <GraduatesModal
          transition={graduatesFor}
          canDelete={permissions.canPurge}
          onClose={() => setGraduatesFor(null)}
          onPurged={() => void refresh()}
        />
      )}

      <ConfirmationModal
        isOpen={revertTarget !== null}
        onClose={() => setRevertTarget(null)}
        onConfirm={handleRevert}
        title="Jahrgangswechsel zurücksetzen"
        confirmText="Ja, zurücksetzen"
        cancelText="Abbrechen"
        isConfirmLoading={busy}
      >
        <p>
          Den Jahrgangswechsel {revertTarget?.academicYear} wirklich
          zurücksetzen? Versetzte Kinder kehren in ihre alte Klasse zurück,
          Abgänge werden wiederhergestellt.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Entwurf löschen"
        confirmText="Ja, löschen"
        cancelText="Abbrechen"
        isConfirmLoading={busy}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover text-white"
      >
        <p>
          Den Entwurf für {deleteTarget?.academicYear} wirklich löschen? Es
          werden keine Kinder verändert.
        </p>
      </ConfirmationModal>
    </div>
  );
}
