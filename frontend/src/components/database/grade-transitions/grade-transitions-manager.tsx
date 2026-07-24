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
  revertGradeTransition,
  type GradeTransition,
  type TransitionResult,
  type TransitionStatus,
} from "~/lib/grade-transition-api";
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
 * backend grade_transitions permission (read/create/update/delete/apply), so a
 * user only ever sees actions the backend will actually allow. Defaults to full
 * access when the caller does not resolve permissions (e.g. isolated tests).
 */
export interface TransitionPermissions {
  readonly canCreate: boolean;
  readonly canUpdate: boolean;
  readonly canDelete: boolean;
  readonly canApply: boolean;
}

const FULL_ACCESS: TransitionPermissions = {
  canCreate: true,
  canUpdate: true,
  canDelete: true,
  canApply: true,
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
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      setTransitions(await listGradeTransitions());
      setLoadError(null);
    } catch (error) {
      logger.error("transitions_load_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setLoadError("Jahrgangswechsel konnten nicht geladen werden.");
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
      toast.success(
        `Jahrgangswechsel zurückgesetzt: ${result.studentsPromoted} Kinder zurückversetzt, ${result.studentsGraduated} Abgänge wiederhergestellt.`,
      );
      setRevertTarget(null);
      void refresh();
    } catch (error) {
      logger.error("revert_failed", {
        transition_id: revertTarget.id,
        error: error instanceof Error ? error.message : String(error),
      });
      toast.error("Zurücksetzen fehlgeschlagen. Bitte erneut versuchen.");
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
      toast.error("Löschen fehlgeschlagen. Bitte erneut versuchen.");
    } finally {
      setBusy(false);
    }
  };

  const openEditorFor = useCallback((transition: GradeTransition | null) => {
    setEditorDraft(transition);
    setEditorOpen(true);
  }, []);

  // Only the most-recently-applied transition may be reverted. Reverting an
  // older one writes each student's recorded from_class back, clobbering the
  // class a later transition (or a manual edit) has since assigned. Gating the
  // action to the latest applied row enforces a strict reverse-order unwind:
  // once it is reverted the previous transition becomes the latest and can be
  // reverted in turn.
  const latestRevertableId = useMemo(() => {
    if (!transitions) return null;
    let latest: GradeTransition | null = null;
    for (const t of transitions) {
      if (!t.canRevert || t.status !== "applied") continue;
      if (latest === null || (t.appliedAt ?? "") > (latest.appliedAt ?? "")) {
        latest = t;
      }
    }
    return latest?.id ?? null;
  }, [transitions]);

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
                className="text-[#FF3130]"
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

      {loadError && <p className="text-sm text-[#FF3130]">{loadError}</p>}

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
        confirmButtonClass="bg-[#FF3130] hover:bg-[#e02c2b] text-white"
      >
        <p>
          Den Entwurf für {deleteTarget?.academicYear} wirklich löschen? Es
          werden keine Kinder verändert.
        </p>
      </ConfirmationModal>
    </div>
  );
}
