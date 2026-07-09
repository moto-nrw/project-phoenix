"use client";

/**
 * SubstitutionSlideOver — the Vertretungsplan (#1840) block editor.
 *
 * Follows the Betreuungsplan InstanceDetailSlideOver pattern for a clear,
 * linear flow: the body shows the block's staffing (base plan vs
 * substitution), block-level actions live in the footer, and every
 * state-changing action is confirmed in a dialog that also holds the optional
 * reason — so nothing fires unexpectedly and there are no stray inline inputs.
 */

import { TriangleAlert, UserMinus, UserX } from "lucide-react";
import { useState } from "react";

import { formatDate } from "~/lib/date-helpers";
import { getActivityTypeBadge, getStatusLabel } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
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

// A pending confirmation. Each variant carries what the dialog needs to render
// and what the confirm handler should run.
type ConfirmAction =
  | { kind: "absent"; staffId: string; staffName: string }
  | { kind: "cancel" }
  | { kind: "unbesetzt" };

function staffLabel(staffNames: Map<string, string>, id: string): string {
  return staffNames.get(id) ?? `Personal #${id}`;
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
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  const [reason, setReason] = useState("");

  const open = instance !== null;
  const canEdit =
    instance?.status === "planned" || instance?.status === "active";

  function openConfirm(action: ConfirmAction) {
    setReason("");
    setConfirm(action);
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  }

  async function handleConfirm() {
    if (!instance || !confirm) return;
    const trimmed = reason.trim() || undefined;
    await run(async () => {
      switch (confirm.kind) {
        case "absent":
          await onMarkAbsent(confirm.staffId, instance.date, trimmed);
          break;
        case "cancel":
          await onCancelBlock(instance, trimmed);
          break;
        case "unbesetzt":
          await onAcknowledge(instance, true, trimmed);
          break;
      }
    });
    setConfirm(null);
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

  const confirmCopy: Record<
    ConfirmAction["kind"],
    { title: string; body: string; confirmText: string; danger?: boolean }
  > = {
    absent: {
      title:
        confirm?.kind === "absent"
          ? `${confirm.staffName} abwesend melden?`
          : "Abwesend melden?",
      body: instance
        ? `Gilt für alle Termine dieser Person am ${formatDate(instance.date)}.`
        : "",
      confirmText: "Abwesend melden",
    },
    cancel: {
      title: "Block absagen?",
      body: "Der Termin wird abgesagt. Die Halbjahresvorlage bleibt unverändert.",
      confirmText: "Block absagen",
      danger: true,
    },
    unbesetzt: {
      title: "Bewusst unbesetzt markieren?",
      body: "Der Block läuft absichtlich ohne Personal und zählt nicht mehr als offene Lücke.",
      confirmText: "Markieren",
    },
  };
  const activeCopy = confirm ? confirmCopy[confirm.kind] : null;

  return (
    <SlideOver
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      {instance && (
        <SlideOverContent
          // The confirmation modal portals to document.body, outside the
          // drawer's DOM. Without these guards Vaul treats a click inside the
          // open modal as an outside-click and closes the drawer (issue #1358).
          onInteractOutside={(event) => {
            if (confirm !== null) event.preventDefault();
          }}
          onEscapeKeyDown={(event) => {
            if (confirm !== null) event.preventDefault();
          }}
        >
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
                  {plannedStaff.map((row) => (
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
                          <Button
                            type="button"
                            variant="outline"
                            size="md"
                            disabled={busy}
                            onClick={() =>
                              openConfirm({
                                kind: "absent",
                                staffId: row.staffId,
                                staffName: staffLabel(staffNames, row.staffId),
                              })
                            }
                          >
                            <UserMinus className="mr-1.5 h-4 w-4" />
                            Abwesend melden
                          </Button>
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
                            disabled={busy || substituteOptions.length === 0}
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
                  ))}
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

              {canEdit && (
                <p className="text-[11px] leading-5 text-gray-400">
                  Abwesenheit und Ersatz gelten für alle Termine dieser Person
                  am {formatDate(instance.date)}.
                </p>
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
              <div className="flex flex-wrap items-center gap-2">
                {isUnstaffedAck ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    disabled={busy}
                    onClick={() => run(() => onAcknowledge(instance, false))}
                  >
                    Unbesetzt aufheben
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    disabled={busy}
                    onClick={() => openConfirm({ kind: "unbesetzt" })}
                  >
                    <TriangleAlert className="mr-1.5 h-4 w-4" />
                    Bewusst unbesetzt
                  </Button>
                )}
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  disabled={busy}
                  onClick={() => openConfirm({ kind: "cancel" })}
                >
                  <UserX className="mr-1.5 h-4 w-4" />
                  Block absagen
                </Button>
              </div>
            </SlideOverFooter>
          )}
        </SlideOverContent>
      )}

      {confirm && activeCopy && (
        <ConfirmationModal
          isOpen
          onClose={() => setConfirm(null)}
          onConfirm={handleConfirm}
          title={activeCopy.title}
          confirmText={activeCopy.confirmText}
          cancelText="Abbrechen"
          isConfirmLoading={busy}
          confirmButtonClass={
            activeCopy.danger
              ? "bg-[#FF3130] hover:bg-[#e02b2a] text-white"
              : undefined
          }
        >
          <div className="space-y-3">
            <p className="text-sm leading-relaxed text-gray-600">
              {activeCopy.body}
            </p>
            <div>
              <label
                htmlFor="deviation-reason"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                Grund (optional)
              </label>
              <input
                id="deviation-reason"
                type="text"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                maxLength={500}
                placeholder="z. B. krank, Fortbildung, Ausflug"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
              />
            </div>
          </div>
        </ConfirmationModal>
      )}
    </SlideOver>
  );
}
