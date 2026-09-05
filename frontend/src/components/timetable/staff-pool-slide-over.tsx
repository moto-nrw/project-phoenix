"use client";

/**
 * StaffPoolSlideOver — Personalpool + atomarer Personal-Move (#1884).
 *
 * Rechtes Panel zum Betreuungsplan-Block: zeigt für das Zeitfenster des
 * Blocks, wer bereits hier eingeplant ist, wer auf einem anderen
 * überlappenden Block steht (verschiebbar), wer laut Dienstplan frei ist
 * (zuweisbar), wer abwesend ist und wer keinen Dienst hat. Verschieben und
 * Zuweisen laufen als EIN atomarer Save über
 * POST /api/timetable/instances/{id}/move-staff.
 *
 * Overlay-Stapelung: das Bestätigungs-Modal (Kit-Modal, z-9999) liegt über
 * dem SlideOver (z-50); solange es offen ist, wird der SlideOver
 * undismissible gestellt, damit Escape nicht beide Ebenen schließt
 * (gleiches Problemfeld wie PR #1962).
 */

import { Fragment, useMemo, useState } from "react";
import { TriangleAlert, UserPlus } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  SlideOver,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useToast } from "~/contexts/ToastContext";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import type {
  EnrichedInstance,
  MoveStaffResponse,
  StaffPoolAssignment,
  StaffPoolEntry,
  StaffPoolResponse,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "StaffPoolSlideOver" });

interface StaffPoolSlideOverProps {
  open: boolean;
  /** Zielblock, der Personal gewinnt. */
  instance: EnrichedInstance | null;
  /** Mutation controls require schedules:manage; the pool itself is readable. */
  canManage: boolean;
  onClose: () => void;
  /** Nach erfolgreichem Move/Zuweisen: Caches revalidieren. */
  onMoved: () => void;
}

interface PendingMove {
  entry: StaffPoolEntry;
  /** Quellblock beim Verschieben; undefined beim Zuweisen aus dem Pool. */
  assignment?: StaffPoolAssignment;
}

export function StaffPoolSlideOver({
  open,
  instance,
  canManage,
  onClose,
  onMoved,
}: StaffPoolSlideOverProps) {
  const toast = useToast();
  const [pendingMove, setPendingMove] = useState<PendingMove | null>(null);
  const [saving, setSaving] = useState(false);

  const instanceId = instance?.id ?? null;
  const {
    data: pool,
    error,
    isLoading,
    mutate,
  } = useSWRAuth(
    open && instanceId ? `timetable-staff-pool-${instanceId}` : null,
    () => timetableService.getStaffPool(instanceId ?? ""),
  );

  const grouped = useMemo(() => groupEntries(pool), [pool]);

  const handleConfirmMove = async () => {
    if (!canManage || !pendingMove || !instanceId) return;
    setSaving(true);
    try {
      const result = await timetableService.moveStaff(instanceId, {
        staffId: pendingMove.entry.staffId,
        sourceInstanceId: pendingMove.assignment?.instanceId,
      });
      announceMoveResult(result, pendingMove, toast);
      setPendingMove(null);
      await mutate();
      onMoved();
    } catch (err) {
      logger.error("staff_move_failed", {
        target_instance_id: instanceId,
        staff_id: pendingMove.entry.staffId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        getApiErrorMessage(
          err,
          "verschieben",
          "Person",
          "Die Person konnte nicht verschoben werden.",
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <SlideOver
        open={open}
        dismissible={!canManage || pendingMove === null}
        onOpenChange={(next) => {
          if (!next) onClose();
        }}
      >
        <SlideOverContent widthClass="sm:w-[460px]">
          <SlideOverHeader>
            <SlideOverTitle>Personalpool</SlideOverTitle>
            <SlideOverDescription>
              {instance
                ? `${instance.title} • ${instance.startTime} – ${instance.endTime}`
                : ""}
            </SlideOverDescription>
          </SlideOverHeader>

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {isLoading && (
              <p className="text-sm text-gray-500">Lade Personalpool …</p>
            )}
            {error && (
              <p className="text-moto-red-strong text-sm">
                Personalpool konnte nicht geladen werden.
              </p>
            )}
            {pool && !pool.dienstplanInUse && (
              <div className="border-moto-orange/30 bg-moto-orange/10 flex items-start gap-2 rounded-xl border p-3 text-xs text-gray-700">
                <TriangleAlert className="text-moto-orange mt-0.5 h-4 w-4 shrink-0" />
                <span>
                  Für diese Woche ist kein Dienstplan gepflegt. Die
                  Verfügbarkeit laut Schichten kann daher nicht angezeigt
                  werden.
                </span>
              </div>
            )}
            {pool && !canManage && (
              <Alert
                type="info"
                message="Sie haben nur Leserechte für den Betreuungsplan. Personal kann nur mit Verwaltungsrechten verschoben oder zugewiesen werden."
              />
            )}
            {pool && (
              // Der Key remountet die Sektionen pro Block: der Auf-/Zuklapp-
              // Zustand einer PoolSection darf nicht vom vorher geöffneten
              // Block in den nächsten durchsickern.
              <Fragment key={instanceId ?? "none"}>
                <PoolSection
                  title="Auf anderen Blöcken"
                  icon={<MotoConceptIcon concept="substitution" size={14} />}
                  emptyText="Niemand ist zeitgleich auf einem anderen Block eingeplant."
                  entries={grouped.assignedElsewhere}
                  renderActions={
                    canManage
                      ? (entry) =>
                          entry.assignments.map((assignment) => (
                            <Button
                              key={assignment.instanceId}
                              type="button"
                              variant="outline"
                              size="compact"
                              disabled={saving}
                              onClick={() =>
                                setPendingMove({ entry, assignment })
                              }
                            >
                              <span className="inline-flex items-center gap-1.5">
                                <MotoConceptIcon
                                  concept="substitution"
                                  size={14}
                                />
                                Hierher verschieben
                              </span>
                            </Button>
                          ))
                      : undefined
                  }
                />
                <PoolSection
                  title="Im Dienst, frei"
                  icon={<MotoConceptIcon concept="staff" size={14} />}
                  emptyText="Niemand ist im Zeitfenster frei im Dienst."
                  entries={grouped.onShiftFree}
                  renderActions={
                    canManage
                      ? (entry) => (
                          <Button
                            type="button"
                            variant="outline"
                            size="compact"
                            disabled={saving}
                            onClick={() => setPendingMove({ entry })}
                          >
                            <span className="inline-flex items-center gap-1.5">
                              <UserPlus className="h-3.5 w-3.5" />
                              Zuweisen
                            </span>
                          </Button>
                        )
                      : undefined
                  }
                />
                <PoolSection
                  title="Bereits auf diesem Block"
                  icon={<MotoConceptIcon concept="staff" size={16} />}
                  emptyText="Noch niemand zugeordnet."
                  entries={grouped.assignedHere}
                />
                <PoolSection
                  title="Abwesend"
                  icon={<MotoConceptIcon concept="staff" size={14} />}
                  emptyText="Keine Abwesenheiten."
                  entries={grouped.absent}
                />
                <PoolSection
                  title="Kein Dienst im Zeitfenster"
                  icon={<MotoConceptIcon concept="closingDays" size={14} />}
                  emptyText="Alle übrigen Personen haben Dienst."
                  entries={grouped.notOnShift}
                  collapsedByDefault
                />
              </Fragment>
            )}
          </div>
        </SlideOverContent>
      </SlideOver>

      {canManage && pendingMove && instance && (
        <ConfirmationModal
          isOpen
          onClose={() => setPendingMove(null)}
          onConfirm={() => void handleConfirmMove()}
          title={
            pendingMove.assignment ? "Person verschieben?" : "Person zuweisen?"
          }
          confirmText={pendingMove.assignment ? "Verschieben" : "Zuweisen"}
          cancelText="Abbrechen"
          isConfirmLoading={saving}
        >
          <p className="text-sm text-gray-600">
            {confirmMessage(pendingMove, instance)}
          </p>
        </ConfirmationModal>
      )}
    </>
  );
}

interface GroupedPool {
  assignedElsewhere: StaffPoolEntry[];
  onShiftFree: StaffPoolEntry[];
  assignedHere: StaffPoolEntry[];
  absent: StaffPoolEntry[];
  notOnShift: StaffPoolEntry[];
}

function groupEntries(pool: StaffPoolResponse | undefined): GroupedPool {
  const grouped: GroupedPool = {
    assignedElsewhere: [],
    onShiftFree: [],
    assignedHere: [],
    absent: [],
    notOnShift: [],
  };
  for (const entry of pool?.entries ?? []) {
    switch (entry.category) {
      case "assigned_elsewhere":
        grouped.assignedElsewhere.push(entry);
        break;
      case "on_shift_free":
        grouped.onShiftFree.push(entry);
        break;
      case "assigned_here":
        grouped.assignedHere.push(entry);
        break;
      case "absent":
        grouped.absent.push(entry);
        break;
      default:
        grouped.notOnShift.push(entry);
    }
  }
  return grouped;
}

function confirmMessage(pending: PendingMove, target: EnrichedInstance) {
  const name = pending.entry.displayName || `Person #${pending.entry.staffId}`;
  if (pending.assignment) {
    return (
      `${name} wird von „${pending.assignment.title}“ ` +
      `(${pending.assignment.startTime} – ${pending.assignment.endTime}) nach ` +
      `„${target.title}“ (${target.startTime} – ${target.endTime}) verschoben. ` +
      `Der Quellblock verliert die Person in demselben Speichervorgang und ` +
      `kann dadurch unterbesetzt werden.`
    );
  }
  return (
    `${name} wird „${target.title}“ ` +
    `(${target.startTime} – ${target.endTime}) zugewiesen.`
  );
}

function announceMoveResult(
  result: MoveStaffResponse,
  pending: PendingMove,
  toast: ReturnType<typeof useToast>,
) {
  const name = pending.entry.displayName || `Person #${pending.entry.staffId}`;
  if (result.action === "already_applied") {
    toast.info(`${name} ist bereits zugeordnet.`);
    return;
  }
  toast.success(
    result.action === "moved"
      ? `${name} wurde verschoben.`
      : `${name} wurde zugewiesen.`,
  );
  // Je Hinweisart EIN Toast (max. drei pro Move), sonst stapeln sich bei
  // mehreren Überlappungen die Meldungen übereinander.
  if (result.coverageWarnings.length > 0) {
    toast.warning(
      result.coverageWarnings.map((warning) => warning.message).join(" · "),
    );
  }
  if (result.timeConflicts.length > 0) {
    const slots = result.timeConflicts
      .map(
        (conflict) =>
          `„${conflict.title}“ (${conflict.startTime} – ${conflict.endTime})`,
      )
      .join(", ");
    toast.warning(`${name} ist zeitgleich auf ${slots} eingeplant.`);
  }
}

interface PoolSectionProps {
  title: string;
  icon: React.ReactNode;
  emptyText: string;
  entries: StaffPoolEntry[];
  renderActions?: (entry: StaffPoolEntry) => React.ReactNode;
  collapsedByDefault?: boolean;
}

function PoolSection({
  title,
  icon,
  emptyText,
  entries,
  renderActions,
  collapsedByDefault = false,
}: PoolSectionProps) {
  const [expanded, setExpanded] = useState(!collapsedByDefault);
  return (
    <div className="space-y-2">
      <button
        type="button"
        className="flex w-full items-center gap-1.5 text-left text-[10px] font-bold tracking-wider text-gray-400 uppercase"
        onClick={() => setExpanded((current) => !current)}
        aria-expanded={expanded}
      >
        <span className="text-gray-400">{icon}</span>
        {title}
        <span className="font-medium text-gray-300">({entries.length})</span>
      </button>
      {expanded &&
        (entries.length === 0 ? (
          <p className="rounded-xl border border-dashed border-gray-200 px-3 py-2 text-xs text-gray-400">
            {emptyText}
          </p>
        ) : (
          <div className="space-y-1.5">
            {entries.map((entry) => (
              <PoolEntryRow
                key={entry.staffId}
                entry={entry}
                actions={renderActions?.(entry)}
              />
            ))}
          </div>
        ))}
    </div>
  );
}

function PoolEntryRow({
  entry,
  actions,
}: {
  entry: StaffPoolEntry;
  actions?: React.ReactNode;
}) {
  const danger = entry.category === "absent";
  const metaParts: string[] = [];
  if (entry.shiftWindows.length > 0) {
    metaParts.push(`Dienst ${entry.shiftWindows.join(", ")}`);
  }
  if (entry.onShift && !entry.coversWindow) {
    metaParts.push("deckt das Zeitfenster nicht voll ab");
  }
  if (entry.absenceReason) {
    metaParts.push(entry.absenceReason);
  }
  for (const assignment of entry.assignments) {
    metaParts.push(
      `${assignment.title} ${assignment.startTime} – ${assignment.endTime}` +
        (assignment.isSubstitute ? " (Ersatz)" : ""),
    );
  }
  return (
    <div
      className={`rounded-xl border p-3 shadow-sm ${
        danger ? "border-moto-red/20 bg-moto-red/10" : "moto-content-surface"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-gray-900">
            {entry.displayName || `Person #${entry.staffId}`}
          </div>
          {metaParts.length > 0 && (
            <div
              className={`mt-0.5 flex items-center gap-1 text-[11px] ${
                danger ? "text-moto-red-strong" : "text-gray-500"
              }`}
            >
              <MotoConceptIcon
                concept="careTimes"
                size={12}
                className="shrink-0"
              />
              <span className="truncate">{metaParts.join(" • ")}</span>
            </div>
          )}
        </div>
        {actions && (
          <div className="flex shrink-0 flex-col items-end gap-1.5">
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}
