"use client";

/**
 * SubstitutionSlideOver — the Vertretungsplan (#1840) block editor.
 *
 * Betreuungsplan-style layout: the body shows the block's staffing (base plan
 * vs substitution) and block-level actions live in a pinned footer. Instead of
 * a modal, a state-changing action reveals an inline confirm right where it was
 * triggered — a short line, an optional reason, and Abbrechen/Bestätigen — so
 * nothing fires unexpectedly and the flow stays in place.
 */

import { TriangleAlert, UserMinus, UserX } from "lucide-react";
import { useState } from "react";

import { formatDate } from "~/lib/date-helpers";
import { getActivityTypeBadge, getStatusLabel } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";

import {
  timetableMutedSurface,
  timetableNestedSurface,
} from "./timetable-style";

export interface StaffOption {
  id: string;
  name: string;
}

interface SubstitutionSlideOverProps {
  instance: EnrichedInstance | null;
  /** Every staff member in the tenant, the substitute picker source. */
  staffOptions: readonly StaffOption[];
  staffNames: Map<string, string>;
  onClose: () => void;
  onMarkAbsent: (
    absentStaffId: string,
    date: string,
    reason?: string,
  ) => Promise<void>;
  onSubstitute: (
    absentStaffId: string,
    substituteStaffId: string,
    date: string,
  ) => Promise<void>;
  onCancelBlock: (instance: EnrichedInstance, reason?: string) => Promise<void>;
  onAcknowledge: (
    instance: EnrichedInstance,
    ack: boolean,
    note?: string,
  ) => Promise<void>;
}

// The action currently awaiting an inline confirm.
type PendingAction =
  | { kind: "absent"; staffId: string }
  | { kind: "cancel" }
  | { kind: "unbesetzt" };

function staffLabel(staffNames: Map<string, string>, id: string): string {
  return staffNames.get(id) ?? `Personal #${id}`;
}

// InlineReasonConfirm — the in-place confirm block that replaces a trigger
// button: an optional hint, an optional reason input, and right-aligned
// Abbrechen/Bestätigen actions.
function InlineReasonConfirm({
  hint,
  confirmText,
  danger,
  busy,
  reason,
  onReasonChange,
  onConfirm,
  onCancel,
}: {
  hint?: string;
  confirmText: string;
  danger?: boolean;
  busy: boolean;
  reason: string;
  onReasonChange: (value: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="space-y-2">
      {hint && <p className="text-xs leading-5 text-gray-500">{hint}</p>}
      <input
        type="text"
        autoFocus
        value={reason}
        onChange={(e) => onReasonChange(e.target.value)}
        maxLength={500}
        placeholder="Grund (optional)"
        className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
      />
      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={busy}
          onClick={onCancel}
        >
          Abbrechen
        </Button>
        <Button
          type="button"
          variant={danger ? "danger" : "primary"}
          size="md"
          isLoading={busy}
          onClick={onConfirm}
        >
          {confirmText}
        </Button>
      </div>
    </div>
  );
}

export function SubstitutionSlideOver({
  instance,
  staffOptions,
  staffNames,
  onClose,
  onMarkAbsent,
  onSubstitute,
  onCancelBlock,
  onAcknowledge,
}: SubstitutionSlideOverProps) {
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [reason, setReason] = useState("");

  const open = instance !== null;
  const canEdit =
    instance?.status === "planned" || instance?.status === "active";
  const locked = pending !== null || busy;

  function start(action: PendingAction) {
    setReason("");
    setPending(action);
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  }

  async function confirmPending() {
    if (!instance || !pending) return;
    const trimmed = reason.trim() || undefined;
    await run(async () => {
      switch (pending.kind) {
        case "absent":
          await onMarkAbsent(pending.staffId, instance.date, trimmed);
          break;
        case "cancel":
          await onCancelBlock(instance, trimmed);
          break;
        case "unbesetzt":
          await onAcknowledge(instance, true, trimmed);
          break;
      }
    });
    setPending(null);
  }

  // Planned staff = the base plan (non-substitute rows). Substitutes are the
  // deviation. Splitting them is exactly the base-vs-Vertretung diff (AC7).
  const plannedStaff = (instance?.staff ?? []).filter((s) => !s.isSubstitute);
  const substitutes = (instance?.staff ?? []).filter((s) => s.isSubstitute);
  const assignedIds = new Set((instance?.staff ?? []).map((s) => s.staffId));

  const substituteOptions = staffOptions
    .filter((s) => !assignedIds.has(s.id))
    .map((s) => ({ value: s.id, label: s.name }));

  const typeBadge = instance
    ? getActivityTypeBadge(instance.activityType)
    : null;
  const isUnstaffedAck = instance?.understaffedAck === true;
  const blockPending =
    pending?.kind === "cancel" || pending?.kind === "unbesetzt";

  return (
    <SlideOver
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      {instance && (
        <SlideOverContent>
          <SlideOverHeader>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <SlideOverTitle>{instance.title}</SlideOverTitle>
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-gray-600 uppercase">
                    {getStatusLabel(instance.status)}
                  </span>
                  {typeBadge && (
                    <span
                      className="rounded-full px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
                      style={{ backgroundColor: typeBadge.bg }}
                    >
                      {typeBadge.label}
                    </span>
                  )}
                </div>
                <SlideOverDescription>
                  {formatDate(instance.date)} • {instance.startTime} –{" "}
                  {instance.endTime} •{" "}
                  {instance.roomName || `Raum #${instance.roomId}`}
                </SlideOverDescription>
              </div>
              <SlideOverCloseButton />
            </div>
          </SlideOverHeader>

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {isUnstaffedAck && (
              <div className="flex items-start gap-2 rounded-xl border border-[#EAB308]/30 bg-[#EAB308]/10 p-3 text-sm text-[#8A6D00]">
                <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <span className="font-semibold">Bewusst unbesetzt.</span>{" "}
                  {instance.understaffedNote}
                </div>
              </div>
            )}

            {/* Base plan vs substitution — the traceable diff (AC7). */}
            <section className="space-y-2">
              <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Geplantes Personal
              </h3>
              {plannedStaff.length === 0 ? (
                <p
                  className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
                >
                  Für diesen Block war niemand geplant.
                </p>
              ) : (
                <ul className="space-y-2">
                  {plannedStaff.map((row) => {
                    const rowPending =
                      pending?.kind === "absent" &&
                      pending.staffId === row.staffId;
                    return (
                      <li
                        key={row.staffId}
                        className={`${timetableNestedSurface} p-3`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span
                            className={`truncate text-sm font-medium ${
                              row.isAbsent
                                ? "text-gray-400 line-through"
                                : "text-gray-900"
                            }`}
                          >
                            {staffLabel(staffNames, row.staffId)}
                          </span>
                          <div className="flex shrink-0 items-center gap-1.5">
                            {row.isPrimary && (
                              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold text-gray-600">
                                Zuständig
                              </span>
                            )}
                            {row.isAbsent && (
                              <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-[10px] font-semibold text-[#CC2626]">
                                Abwesend
                              </span>
                            )}
                          </div>
                        </div>

                        {row.isAbsent && row.absenceReason && (
                          <p className="mt-1 text-xs text-gray-500">
                            Grund: {row.absenceReason}
                          </p>
                        )}

                        {canEdit && !row.isAbsent && (
                          <div className="mt-2">
                            {rowPending ? (
                              <InlineReasonConfirm
                                hint={`Gilt für alle Termine dieser Person am ${formatDate(instance.date)}.`}
                                confirmText="Abwesend melden"
                                busy={busy}
                                reason={reason}
                                onReasonChange={setReason}
                                onConfirm={confirmPending}
                                onCancel={() => setPending(null)}
                              />
                            ) : (
                              <Button
                                type="button"
                                variant="outline"
                                size="md"
                                disabled={locked}
                                onClick={() =>
                                  start({
                                    kind: "absent",
                                    staffId: row.staffId,
                                  })
                                }
                              >
                                <UserMinus className="mr-1.5 h-4 w-4" />
                                Abwesend melden
                              </Button>
                            )}
                          </div>
                        )}

                        {canEdit && row.isAbsent && (
                          <div className="mt-2">
                            <span className="mb-1 block text-[11px] font-medium text-gray-500">
                              Ersatz eintragen
                            </span>
                            <CustomSelect
                              value=""
                              placeholder="Ersatzperson wählen…"
                              options={substituteOptions}
                              ariaLabel={`Ersatz für ${staffLabel(staffNames, row.staffId)}`}
                              disabled={
                                locked || substituteOptions.length === 0
                              }
                              onChange={(sub) => {
                                if (!sub) return;
                                void run(() =>
                                  onSubstitute(row.staffId, sub, instance.date),
                                );
                              }}
                            />
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}

              {substitutes.length > 0 && (
                <div className="space-y-1 pt-1">
                  <h4 className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
                    Vertretung
                  </h4>
                  <ul className="space-y-1">
                    {substitutes.map((row) => (
                      <li
                        key={row.staffId}
                        className="flex items-center justify-between gap-2 rounded-lg bg-[#83CD2D]/10 px-3 py-2"
                      >
                        <span className="truncate text-sm font-medium text-gray-900">
                          {staffLabel(staffNames, row.staffId)}
                        </span>
                        <span className="rounded-full bg-[#83CD2D]/20 px-2 py-0.5 text-[10px] font-semibold text-[#5A8E1F]">
                          Ersatz
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </section>

            {!canEdit && (
              <p
                className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
              >
                Vergangene oder abgeschlossene Termine können nicht mehr
                geändert werden.
              </p>
            )}
          </div>

          {canEdit && (
            <SlideOverFooter>
              {blockPending ? (
                <InlineReasonConfirm
                  hint={
                    pending?.kind === "cancel"
                      ? "Der Termin wird abgesagt. Die Halbjahresvorlage bleibt unverändert."
                      : "Der Block läuft absichtlich ohne Personal und zählt nicht mehr als offene Lücke."
                  }
                  confirmText={
                    pending?.kind === "cancel" ? "Block absagen" : "Markieren"
                  }
                  danger={pending?.kind === "cancel"}
                  busy={busy}
                  reason={reason}
                  onReasonChange={setReason}
                  onConfirm={confirmPending}
                  onCancel={() => setPending(null)}
                />
              ) : (
                <div className="flex flex-wrap items-center justify-end gap-2">
                  {isUnstaffedAck ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={locked}
                      onClick={() => run(() => onAcknowledge(instance, false))}
                    >
                      Unbesetzt aufheben
                    </Button>
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={locked}
                      onClick={() => start({ kind: "unbesetzt" })}
                    >
                      <TriangleAlert className="mr-1.5 h-4 w-4" />
                      Bewusst unbesetzt
                    </Button>
                  )}
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="md"
                    disabled={locked}
                    onClick={() => start({ kind: "cancel" })}
                  >
                    <UserX className="mr-1.5 h-4 w-4" />
                    Block absagen
                  </Button>
                </div>
              )}
            </SlideOverFooter>
          )}
        </SlideOverContent>
      )}
    </SlideOver>
  );
}
