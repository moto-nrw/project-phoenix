"use client";

import { useState } from "react";
import { Modal } from "../ui/modal";
import { TimePicker } from "../ui/time-picker";
import { createScheduledCheckout, performImmediateCheckout } from "~/lib/scheduled-checkout-api";
import { useSession } from "next-auth/react";

interface ScheduledCheckoutModalProps {
  isOpen: boolean;
  onClose: () => void;
  studentId: string;
  studentName: string;
  onCheckoutScheduled: () => void;
}

export function ScheduledCheckoutModal({
  isOpen,
  onClose,
  studentId,
  studentName,
  onCheckoutScheduled,
}: ScheduledCheckoutModalProps) {
  const { data: session } = useSession();
  const [checkoutTime, setCheckoutTime] = useState<string>("");
  const [checkoutType, setCheckoutType] = useState<"now" | "scheduled">("now");
  const [reason, setReason] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    setError(null);
    setIsSubmitting(true);

    try {
      let scheduledFor: string;
      
      if (checkoutType === "now") {
        // Perform immediate checkout instead of scheduling
        try {
          await performImmediateCheckout(parseInt(studentId, 10), session?.user?.token);
          onCheckoutScheduled();
          onClose();
          
          // Reset form
          setCheckoutTime("");
          setCheckoutType("now");
          setReason("");
          return;
        } catch (error) {
          console.error("Error performing immediate checkout:", error);
          setError("Fehler beim sofortigen Checkout");
          setIsSubmitting(false);
          return;
        }
      } else {
        if (!checkoutTime) {
          setError("Bitte wählen Sie eine Uhrzeit aus");
          setIsSubmitting(false);
          return;
        }
        
        // Create a date object for today with the selected time
        const now = new Date();
        const [hours, minutes] = checkoutTime.split(":");
        const scheduledDate = new Date(
          now.getFullYear(),
          now.getMonth(),
          now.getDate(),
          parseInt(hours ?? "0", 10),
          parseInt(minutes ?? "0", 10)
        );
        
        // Check if the time is in the past
        if (scheduledDate <= now) {
          setError("Die ausgewählte Zeit liegt in der Vergangenheit");
          setIsSubmitting(false);
          return;
        }
        
        scheduledFor = scheduledDate.toISOString();
      }

      await createScheduledCheckout(
        {
          student_id: parseInt(studentId, 10),
          scheduled_for: scheduledFor,
          reason: reason || undefined,
        },
        session?.user?.token
      );

      onCheckoutScheduled();
      onClose();
      
      // Reset form
      setCheckoutTime("");
      setCheckoutType("now");
      setReason("");
    } catch (error) {
      console.error("Error scheduling checkout:", error);
      setError("Fehler beim Planen des Checkouts");
    } finally {
      setIsSubmitting(false);
    }
  };

  const modalFooter = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={isSubmitting}
        className="flex-1 px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 whitespace-nowrap"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={handleSubmit}
        disabled={isSubmitting}
        className="flex-1 px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 whitespace-nowrap"
      >
        {isSubmitting ? "Wird verarbeitet..." : checkoutType === "now" ? "Jetzt ausloggen" : "Ausloggen planen"}
      </button>
    </>
  );

  // Get current time for min attribute
  const now = new Date();
  const currentTime = `${now.getHours().toString().padStart(2, "0")}:${now
    .getMinutes()
    .toString()
    .padStart(2, "0")}`;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Checkout für ${studentName}`}
      footer={modalFooter}
    >
      <div className="space-y-5">
        {/* Checkout Type Selection */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-3">
            Checkout-Zeitpunkt
          </label>
          <div className="space-y-2">
            <label className="flex items-center gap-3 p-3 border border-gray-200 rounded-lg cursor-pointer hover:bg-gray-50 transition-colors">
              <input
                type="radio"
                value="now"
                checked={checkoutType === "now"}
                onChange={(e) => setCheckoutType(e.target.value as "now")}
                className="h-4 w-4 text-gray-900 focus:ring-gray-900"
              />
              <span className="text-sm text-gray-900">Sofort ausloggen</span>
            </label>
            <label className="flex items-center gap-3 p-3 border border-gray-200 rounded-lg cursor-pointer hover:bg-gray-50 transition-colors">
              <input
                type="radio"
                value="scheduled"
                checked={checkoutType === "scheduled"}
                onChange={(e) => setCheckoutType(e.target.value as "scheduled")}
                className="h-4 w-4 text-gray-900 focus:ring-gray-900"
              />
              <span className="text-sm text-gray-900">Zu einer bestimmten Zeit</span>
            </label>
          </div>
        </div>

        {/* Time Selection (only shown if scheduled is selected) */}
        {checkoutType === "scheduled" && (
          <div>
            <label
              htmlFor="checkout-time"
              className="block text-sm font-medium text-gray-700 mb-3"
            >
              Uhrzeit auswählen
            </label>
            <TimePicker
              value={checkoutTime || currentTime}
              onChange={setCheckoutTime}
              min={currentTime}
              className="mt-2"
            />
            <p className="mt-2 text-xs text-gray-500 text-center">
              Verwenden Sie die Pfeile oder tippen Sie direkt auf die Zahlen
            </p>
          </div>
        )}

        {/* Reason (optional) */}
        <div>
          <label
            htmlFor="reason"
            className="block text-sm font-medium text-gray-700 mb-2"
          >
            Grund (optional)
          </label>
          <textarea
            id="reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            className="block w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-900 focus:ring-1 focus:ring-gray-900 transition-colors resize-none"
            placeholder="z.B. Arzttermin, früher abholen..."
          />
        </div>

        {/* Error Message */}
        {error && (
          <div className="rounded-lg bg-red-50 border border-red-200 p-3">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Info Message */}
        <div className="rounded-lg bg-gray-50 border border-gray-200 p-3">
          <p className="text-sm text-gray-700">
            {checkoutType === "now"
              ? "Der Schüler wird in wenigen Momenten ausgecheckt."
              : "Der Schüler wird zur angegebenen Zeit automatisch ausgecheckt."}
          </p>
        </div>
      </div>
    </Modal>
  );
}