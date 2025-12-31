"use client";

import { ScheduledCheckoutInfo } from "~/components/scheduled-checkout/scheduled-checkout-info";

interface StudentCheckoutSectionProps {
  studentId: string;
  hasScheduledCheckout: boolean;
  onUpdate: () => void;
  onScheduledCheckoutChange: (hasCheckout: boolean) => void;
  onCheckoutClick: () => void;
}

export function StudentCheckoutSection({
  studentId,
  hasScheduledCheckout,
  onUpdate,
  onScheduledCheckoutChange,
  onCheckoutClick,
}: StudentCheckoutSectionProps) {
  return (
    <div className="mb-6 rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
            />
          </svg>
        </div>
        <h3 className="text-base font-semibold text-gray-900 sm:text-lg">
          Abmeldung
        </h3>
      </div>
      <ScheduledCheckoutInfo
        studentId={studentId}
        onUpdate={onUpdate}
        onScheduledCheckoutChange={onScheduledCheckoutChange}
      />
      {!hasScheduledCheckout && (
        <button
          onClick={onCheckoutClick}
          className="mt-4 flex w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 py-3 text-sm font-medium text-white transition-all duration-200 hover:scale-[1.01] hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:py-2.5"
        >
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
            />
          </svg>
          Kind abmelden
        </button>
      )}
    </div>
  );
}
