"use client";

import { useEffect, useState, useCallback } from "react";
// Format time as HH:mm
function formatTime(date: Date): string {
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  return `${hours}:${minutes}`;
}
import { Button } from "../ui/button";
import { cancelScheduledCheckout, getStudentScheduledCheckouts } from "~/lib/scheduled-checkout-api";
import type { ScheduledCheckout } from "~/lib/scheduled-checkout-api";
import { useSession } from "next-auth/react";

interface ScheduledCheckoutInfoProps {
  studentId: string;
  onUpdate?: () => void;
  onScheduledCheckoutChange?: (hasScheduledCheckout: boolean) => void;
}

export function ScheduledCheckoutInfo({ studentId, onUpdate, onScheduledCheckoutChange }: ScheduledCheckoutInfoProps) {
  const { data: session } = useSession();
  const [scheduledCheckouts, setScheduledCheckouts] = useState<ScheduledCheckout[]>([]);
  const [loading, setLoading] = useState(true);
  const [cancelling, setCancelling] = useState<number | null>(null);

  const fetchScheduledCheckouts = useCallback(async () => {
    try {
      const checkouts = await getStudentScheduledCheckouts(studentId, session?.user?.token);
      // Filter only pending checkouts
      const pendingCheckouts = checkouts.filter(c => c.status === "pending");
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
      await cancelScheduledCheckout(checkoutId.toString(), session?.user?.token);
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
          className="rounded-lg bg-amber-50 border border-amber-200 p-4"
        >
          <div className="flex items-start justify-between">
            <div>
              <div className="flex items-center gap-2">
                <svg
                  className="h-5 w-5 text-amber-600"
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
                <h4 className="text-sm font-medium text-amber-800">
                  Geplanter Checkout
                </h4>
              </div>
              <p className="mt-1 text-sm text-amber-700">
                {formatTime(new Date(checkout.scheduled_for))} Uhr
                {checkout.reason && ` - ${checkout.reason}`}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCancel(checkout.id)}
              disabled={cancelling === checkout.id}
              className="text-amber-700 hover:text-amber-800 border-amber-300 hover:border-amber-400"
            >
              {cancelling === checkout.id ? "Wird storniert..." : "Stornieren"}
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}