"use client";

import { useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import type {
  PickupException,
  PickupExceptionFormData,
} from "@/lib/pickup-schedule-helpers";
import { formatPickupTime } from "@/lib/pickup-schedule-helpers";

interface PickupExceptionFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmit: (data: PickupExceptionFormData) => Promise<void>;
  readonly initialData?: PickupException;
  readonly mode: "create" | "edit";
}

export function PickupExceptionFormModal({
  isOpen,
  onClose,
  onSubmit,
  initialData,
  mode,
}: PickupExceptionFormModalProps) {
  const [exceptionDate, setExceptionDate] = useState("");
  const [pickupTime, setPickupTime] = useState("");
  const [noPickup, setNoPickup] = useState(false);
  const [reason, setReason] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      if (initialData) {
        setExceptionDate(initialData.exceptionDate);
        setPickupTime(
          initialData.pickupTime
            ? formatPickupTime(initialData.pickupTime)
            : "",
        );
        setNoPickup(!initialData.pickupTime);
        setReason(initialData.reason);
      } else {
        // Default to tomorrow for new exceptions
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        setExceptionDate(tomorrow.toISOString().split("T")[0] ?? "");
        setPickupTime("");
        setNoPickup(false);
        setReason("");
      }
      setError(null);
    }
  }, [isOpen, initialData]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!exceptionDate) {
      setError("Bitte wählen Sie ein Datum aus.");
      return;
    }

    if (!noPickup && !pickupTime) {
      setError(
        "Bitte geben Sie eine Abholzeit an oder aktivieren Sie 'Kein Abholen'.",
      );
      return;
    }

    if (!reason.trim()) {
      setError("Bitte geben Sie einen Grund an.");
      return;
    }

    if (reason.length > 255) {
      setError("Der Grund darf maximal 255 Zeichen lang sein.");
      return;
    }

    // Validate time format if provided
    if (!noPickup && pickupTime) {
      const timeRegex = /^([01]?[0-9]|2[0-3]):[0-5][0-9]$/;
      if (!timeRegex.test(pickupTime)) {
        setError("Ungültiges Zeitformat. Bitte verwenden Sie HH:MM.");
        return;
      }
    }

    setIsSubmitting(true);
    try {
      await onSubmit({
        exceptionDate,
        pickupTime: noPickup ? undefined : pickupTime,
        reason: reason.trim(),
      });
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Speichern der Ausnahme",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
        disabled={isSubmitting}
      >
        Abbrechen
      </button>
      <button
        type="submit"
        form="pickup-exception-form"
        disabled={isSubmitting}
        className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
      >
        {isSubmitting && <Loader2 className="h-4 w-4 animate-spin" />}
        {mode === "create" ? "Hinzufügen" : "Speichern"}
      </button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={mode === "create" ? "Neue Ausnahme" : "Ausnahme bearbeiten"}
      footer={footer}
      size="sm"
    >
      <form id="pickup-exception-form" onSubmit={handleSubmit}>
        <div className="space-y-4">
          {error && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}

          {/* Date */}
          <div>
            <label
              htmlFor="exception-date"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Datum
            </label>
            <input
              id="exception-date"
              type="date"
              value={exceptionDate}
              onChange={(e) => setExceptionDate(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              min={new Date().toISOString().split("T")[0]}
              required
            />
          </div>

          {/* No Pickup Checkbox */}
          <div className="flex items-center gap-2">
            <input
              id="no-pickup"
              type="checkbox"
              checked={noPickup}
              onChange={(e) => {
                setNoPickup(e.target.checked);
                if (e.target.checked) {
                  setPickupTime("");
                }
              }}
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <label
              htmlFor="no-pickup"
              className="text-sm font-medium text-gray-700"
            >
              Kein Abholen an diesem Tag (z.B. krank, frei)
            </label>
          </div>

          {/* Time (only if not "no pickup") */}
          {!noPickup && (
            <div>
              <label
                htmlFor="pickup-time"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Abholzeit
              </label>
              <input
                id="pickup-time"
                type="time"
                value={pickupTime}
                onChange={(e) => setPickupTime(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                required={!noPickup}
              />
            </div>
          )}

          {/* Reason */}
          <div>
            <label
              htmlFor="reason"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Grund
            </label>
            <input
              id="reason"
              type="text"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              placeholder="z.B. Arzttermin, Familienfeier"
              maxLength={255}
              required
            />
            <p className="mt-1 text-xs text-gray-500">
              {reason.length}/255 Zeichen
            </p>
          </div>
        </div>
      </form>
    </FormModal>
  );
}
