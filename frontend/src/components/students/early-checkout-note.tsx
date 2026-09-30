"use client";

import { useCallback, useState, type ReactNode } from "react";
import { ConfirmationModal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { earlyCheckoutMinutes } from "~/lib/student-time-status";
import { useTenantSafe } from "~/lib/tenant-context";

/** Matches the backend limit (studentpresence.MaxCheckoutNoteLength). */
const MAX_CHECKOUT_NOTE_LENGTH = 500;

/**
 * Tells whether a checkout right now would be early (#3324): more than the
 * school's tolerance before today's pickup time. Always false when the school
 * switched the question off or the child has no pickup time today.
 */
export function useEarlyCheckoutCheck(): (
  plannedPickup?: string | null,
) => boolean {
  const tenant = useTenantSafe();
  const tolerance = tenant?.tenant?.earlyCheckoutNoteToleranceMinutes ?? null;
  return useCallback(
    (plannedPickup?: string | null) =>
      earlyCheckoutMinutes({
        plannedPickup,
        now: new Date(),
        toleranceMinutes: tolerance,
      }) !== null,
    [tolerance],
  );
}

/**
 * Optional reason field for an early checkout. It names today's pickup time,
 * so staff see why moto asks, and says the reason is optional.
 */
export function EarlyCheckoutNoteField({
  id,
  plannedPickup,
  value,
  onChange,
}: Readonly<{
  id: string;
  plannedPickup: string;
  value: string;
  onChange: (value: string) => void;
}>) {
  return (
    <div className="mt-4 space-y-3">
      <p className="text-sm text-gray-600">
        Die Abholzeit heute ist{" "}
        <span className="font-medium text-gray-900">
          {plannedPickup.slice(0, 5)} Uhr
        </span>
        .
      </p>
      <Textarea
        id={id}
        name="checkout_note"
        label="Grund für das frühe Gehen (freiwillig)"
        rows={2}
        maxLength={MAX_CHECKOUT_NOTE_LENGTH}
        placeholder="Zum Beispiel: Arzttermin"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      <p className="text-xs text-gray-500">Der Grund steht danach beim Kind.</p>
    </div>
  );
}

export interface EarlyCheckoutTarget {
  studentId: string;
  studentName: string;
  plannedPickup?: string | null;
  /** Room whose visit the checkout ends (detailed mode), else null. */
  room: string | null;
}

/**
 * Checkout dialog for the one-tap surfaces (Kindersuche, OGS-Gruppen). A
 * checkout that is not early stays a single tap: `request` returns false and
 * the caller continues as before.
 */
export function useEarlyCheckoutDialog(
  onConfirm: (studentId: string, note: string) => void,
): {
  request: (target: EarlyCheckoutTarget) => boolean;
  dialog: ReactNode;
} {
  const isEarly = useEarlyCheckoutCheck();
  const [target, setTarget] = useState<
    (EarlyCheckoutTarget & { plannedPickup: string }) | null
  >(null);
  const [note, setNote] = useState("");

  const request = useCallback(
    (next: EarlyCheckoutTarget) => {
      if (!next.plannedPickup || !isEarly(next.plannedPickup)) return false;
      setNote("");
      setTarget({ ...next, plannedPickup: next.plannedPickup });
      return true;
    },
    [isEarly],
  );

  const dialog = (
    <ConfirmationModal
      isOpen={target !== null}
      onClose={() => setTarget(null)}
      onConfirm={() => {
        if (!target) return;
        onConfirm(target.studentId, note);
        setTarget(null);
      }}
      title="Kind früher abmelden"
      confirmText="Abmelden"
      confirmVariant={target?.room ? "danger" : "primary"}
    >
      <p className="text-sm text-gray-600">
        Möchten Sie{" "}
        <span className="font-medium text-gray-900">{target?.studentName}</span>{" "}
        jetzt abmelden?
        {target?.room ? (
          <>
            {" "}
            Das Kind ist gerade in{" "}
            <span className="font-medium text-gray-900">{target.room}</span>.
            Der Raumbesuch endet beim Abmelden.
          </>
        ) : null}
      </p>
      {target ? (
        <EarlyCheckoutNoteField
          id="early-checkout-note"
          plannedPickup={target.plannedPickup}
          value={note}
          onChange={setNote}
        />
      ) : null}
    </ConfirmationModal>
  );

  return { request, dialog };
}
