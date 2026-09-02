"use client";

// Begründungs-Modal für den Monatsabschluss (#1417): Abschließen (schulweit)
// und Wiedereröffnen (pro Person) tragen beide eine Pflicht-Begründung.
// Folgt dem etablierten Muster von ResetModal/AdjustmentModal im
// Stundenkonto-Panel: Modal + Pflicht-Textarea + canSubmit-Gate.

import { useState, type ReactNode } from "react";

import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { useToast } from "~/contexts/ToastContext";

export function MonthCloseReasonModal({
  title,
  description,
  submitLabel,
  successMessage,
  destructive = false,
  onSubmit,
  onClose,
}: {
  readonly title: string;
  /** Erklärt, was die Aktion tut und wann sie der richtige Weg ist. */
  readonly description: ReactNode;
  readonly submitLabel: string;
  /** Optional: Erfolgs-Toast; weglassen, wenn der Aufrufer selbst meldet. */
  readonly successMessage?: string;
  /** Reopen ist die Ausnahme-Aktion und bekommt die rote Variante. */
  readonly destructive?: boolean;
  readonly onSubmit: (reason: string) => Promise<void>;
  readonly onClose: () => void;
}) {
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const canSubmit = !submitting && reason.trim() !== "";

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      await onSubmit(reason.trim());
      if (successMessage) toast.success(successMessage);
      onClose();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Aktion fehlgeschlagen.",
      );
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title={title}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant={destructive ? "danger" : "primary"}
            size="md"
            onClick={handleSubmit}
            disabled={!canSubmit}
          >
            {submitting ? "Bitte warten…" : submitLabel}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="space-y-2 text-sm text-gray-600">{description}</div>
        <div>
          <label
            htmlFor="month-close-reason"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Begründung (Pflicht)
          </label>
          <Textarea
            id="month-close-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={2}
            placeholder="z. B. Monatsabschluss für die Lohnabrechnung"
          />
        </div>
      </div>
    </Modal>
  );
}
