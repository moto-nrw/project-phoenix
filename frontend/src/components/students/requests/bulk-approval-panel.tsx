"use client";

/**
 * Sammelfreigabe (#2432): mehrere Anfragen eines Kindes mit einer Begründung
 * gemeinsam freigeben. Klappt eine nicht, wird keine freigegeben.
 */

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { SectionCard } from "~/components/ui/section-card";

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
    <SectionCard
      title="Gemeinsam freigeben"
      description={
        count === 1
          ? "Wählen Sie noch eine passende Anfrage aus."
          : "Alle ausgewählten Anfragen werden freigegeben. Klappt eine nicht, wird keine freigegeben."
      }
      bodyClassName="compact:mt-3 mt-4 space-y-3"
    >
      <Textarea
        label={
          reasonRequired
            ? "Gemeinsame Begründung"
            : "Gemeinsame Begründung (freiwillig)"
        }
        id="bulk-approval-reason"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        rows={2}
        placeholder="Warum können diese Anfragen freigegeben werden?"
      />
      <Button
        type="button"
        variant="primary"
        size="md"
        disabled={blocked}
        onClick={open}
      >
        {count} {count === 1 ? "Anfrage" : "Anfragen"} freigeben
      </Button>
    </SectionCard>
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
      loadingText="Wird freigegeben …"
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
