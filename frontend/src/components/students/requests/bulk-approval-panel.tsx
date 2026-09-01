"use client";

/**
 * Sammelfreigabe (#2432): mehrere Anfragen eines Kindes mit einer Begründung
 * gemeinsam freigeben. Klappt eine nicht, wird keine freigegeben.
 */

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";

export function BulkApprovalPanel({
  count,
  reason,
  setReason,
  reasonRequired,
  open,
}: Readonly<{
  count: number;
  reason: string;
  setReason: (value: string) => void;
  /** Verlangt die Schule eine Begründung? Sonst ist sie freiwillig. */
  reasonRequired: boolean;
  open: () => void;
}>) {
  if (count === 0) return null;
  const blocked = count < 2 || (reasonRequired && reason.trim() === "");
  return (
    <div className="moto-content-surface space-y-3 rounded-2xl border p-4 shadow-md">
      <div>
        <h2 className="font-semibold text-gray-900">Gemeinsam freigeben</h2>
        <p className="text-sm text-gray-600">
          {count === 1
            ? "Wählen Sie noch eine passende Anfrage aus."
            : "Alle ausgewählten Anfragen werden freigegeben. Klappt eine nicht, wird keine freigegeben."}
        </p>
      </div>
      <label
        htmlFor="bulk-approval-reason"
        className="block space-y-1 text-sm font-medium text-gray-800"
      >
        <span>
          {reasonRequired
            ? "Gemeinsame Begründung"
            : "Gemeinsame Begründung (freiwillig)"}
        </span>
        <Textarea
          id="bulk-approval-reason"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={2}
          placeholder="Warum können diese Anfragen freigegeben werden?"
        />
      </label>
      <Button
        type="button"
        variant="primary"
        size="md"
        className="max-sm:min-h-11"
        disabled={blocked}
        onClick={open}
      >
        {count} {count === 1 ? "Anfrage" : "Anfragen"} freigeben
      </Button>
    </div>
  );
}

export function BulkConfirmationDialog({
  open,
  count,
  reason,
  saving,
  close,
  confirm,
}: Readonly<{
  open: boolean;
  count: number;
  reason: string;
  saving: boolean;
  close: () => void;
  confirm: () => void;
}>) {
  if (!open) return null;
  return (
    <ConfirmationModal
      isOpen
      onClose={close}
      onConfirm={confirm}
      title={`${count} Anfragen gemeinsam freigeben?`}
      confirmText={`${count} Anfragen freigeben`}
      cancelText="Zurück"
      isConfirmLoading={saving}
      isDismissDisabled={saving}
      mobileSheet
    >
      <div className="space-y-2 text-sm text-gray-700">
        <p>
          Alle {count} Anfragen werden gemeinsam freigegeben. Wenn eine Anfrage
          nicht mehr passt, wird keine freigegeben.
        </p>
        {reason.trim() !== "" && (
          <p>
            <span className="font-medium">Begründung:</span> {reason.trim()}
          </p>
        )}
      </div>
    </ConfirmationModal>
  );
}
