"use client";

/**
 * LEGACY: Displays and manages existing scheduled checkouts.
 * New scheduled checkouts can no longer be created (modal removed Dec 2025).
 * This component remains to:
 * - Display any existing pending checkouts from before the feature was disabled
 * - Allow cancellation of existing scheduled checkouts
 * TODO: Remove once confirmed no scheduled checkouts exist in production.
 */

import { useEffect, useState, useCallback } from "react";
import {
  cancelScheduledCheckout,
  getStudentScheduledCheckouts,
} from "~/lib/scheduled-checkout-api";
import type { ScheduledCheckout } from "~/lib/scheduled-checkout-api";
import { useSession } from "next-auth/react";

// Format time as HH:mm
function formatTime(date: Date): string {
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  return `${hours}:${minutes}`;
}

interface ScheduledCheckoutInfoProps {
  readonly studentId: string;
  readonly onUpdate?: () => void;
  readonly onScheduledCheckoutChange?: (hasScheduledCheckout: boolean) => void;
}

export function ScheduledCheckoutInfo({
  studentId,
  onUpdate,
  onScheduledCheckoutChange,
}: ScheduledCheckoutInfoProps) {
  const { data: session } = useSession();
  const [scheduledCheckouts, setScheduledCheckouts] = useState<
    ScheduledCheckout[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [cancelling, setCancelling] = useState<number | null>(null);

  const fetchScheduledCheckouts = useCallback(async () => {
    try {
      const checkouts = await getStudentScheduledCheckouts(
        studentId,
        session?.user?.token,
      );
      // Filter only pending checkouts
      const pendingCheckouts = checkouts.filter((c) => c.status === "pending");
      setScheduledCheckouts(pendingCheckouts);
      // Notify parent component about the scheduled checkout state
      onScheduledCheckoutChange?.(pendingCheckouts.length > 0);
    } catch (error) {
      console.error("Error fetching scheduled checkouts:", error);
    } finally {
      setLoading(false);
    }
  }, [studentId, session?.user?.token, onScheduledCheckoutChange]);

  useEffect(() => {
    void fetchScheduledCheckouts();
  }, [studentId, session?.user?.token, fetchScheduledCheckouts]);

  const handleCancel = async (checkoutId: number) => {
    setCancelling(checkoutId);
    try {
      await cancelScheduledCheckout(
        checkoutId.toString(),
        session?.user?.token,
      );
      await fetchScheduledCheckouts();
      onUpdate?.();
    } catch (error) {
      console.error("Error cancelling scheduled checkout:", error);
    } finally {
      setCancelling(null);
    }
  };

  if (loading) {
    return null;
  }

  if (scheduledCheckouts.length === 0) {
    return null;
  }

  return (
    <div className="space-y-3">
      {scheduledCheckouts.map((checkout) => (
        <div
          key={checkout.id}
          className="rounded-lg border border-gray-200 bg-gray-50 p-4"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <div className="mb-1 flex items-center gap-2">
                <svg
                  className="h-5 w-5 flex-shrink-0 text-gray-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <h4 className="text-sm font-medium text-gray-900">
                  Geplanter Checkout
                </h4>
              </div>
              <p className="ml-7 text-sm text-gray-600">
                {formatTime(new Date(checkout.scheduled_for))} Uhr
                {checkout.reason && ` - ${checkout.reason}`}
              </p>
            </div>
            <button
              onClick={() => handleCancel(checkout.id)}
              disabled={cancelling === checkout.id}
              className="flex-shrink-0 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-sm active:scale-100 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {cancelling === checkout.id ? "Wird storniert..." : "Stornieren"}
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
