"use client";

import { useState } from "react";
import { Modal } from "../ui/modal";
import { Button } from "../ui/button";
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
      <Button variant="outline" onClick={onClose} disabled={isSubmitting}>
        Abbrechen
      </Button>
      <Button onClick={handleSubmit} disabled={isSubmitting}>
        {isSubmitting ? "Wird geplant..." : "Checkout planen"}
      </Button>
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
      <div className="space-y-6">
        {/* Checkout Type Selection */}
        <div>
          <label className="text-sm font-medium text-gray-700">
            Checkout-Zeitpunkt
          </label>
          <div className="mt-2 space-y-2">
            <label className="flex items-center">
              <input
                type="radio"
                value="now"
                checked={checkoutType === "now"}
                onChange={(e) => setCheckoutType(e.target.value as "now")}
                className="mr-2"
              />
              <span>Sofort</span>
            </label>
            <label className="flex items-center">
              <input
                type="radio"
                value="scheduled"
                checked={checkoutType === "scheduled"}
                onChange={(e) => setCheckoutType(e.target.value as "scheduled")}
                className="mr-2"
              />
              <span>Zu einer bestimmten Zeit</span>
            </label>
          </div>
        </div>

        {/* Time Selection (only shown if scheduled is selected) */}
        {checkoutType === "scheduled" && (
          <div>
            <label
              htmlFor="checkout-time"
              className="block text-sm font-medium text-gray-700"
            >
              Uhrzeit
            </label>
            <input
              type="time"
              id="checkout-time"
              value={checkoutTime}
              onChange={(e) => setCheckoutTime(e.target.value)}
              min={currentTime}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              required
            />
          </div>
        )}

        {/* Reason (optional) */}
        <div>
          <label
            htmlFor="reason"
            className="block text-sm font-medium text-gray-700"
          >
            Grund (optional)
          </label>
          <textarea
            id="reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            placeholder="z.B. Arzttermin, früher abholen..."
          />
        </div>

        {/* Error Message */}
        {error && (
          <div className="rounded-md bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Info Message */}
        <div className="rounded-md bg-blue-50 p-4">
          <p className="text-sm text-blue-800">
            {checkoutType === "now"
              ? "Der Schüler wird in wenigen Momenten ausgecheckt."
              : "Der Schüler wird zur angegebenen Zeit automatisch ausgecheckt."}
          </p>
        </div>
      </div>
    </Modal>
  );
}