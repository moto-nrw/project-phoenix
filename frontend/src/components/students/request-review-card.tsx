"use client";

import { type ReactNode, useState } from "react";
import { ChevronDown } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";

/**
 * One pending parent change request in the staff Änderungsanfragen queue, in the
 * calm card look of the parent portal. Collapsed by default so a long queue
 * stays scannable: the row shows the child + a one-line summary of what changed;
 * clicking it expands the full "current → requested" diff (passed as children),
 * an optional reason, and Freigeben / Ablehnen. Shared by the master-data and
 * care-schedule review lists. Color is intentionally sparse — monochrome
 * buttons, no filled green/red.
 */
export function RequestReviewCard({
  childName,
  summary,
  children,
  reason,
  onReasonChange,
  reasonPlaceholder,
  reasonError,
  busy,
  onApprove,
  onReject,
}: Readonly<{
  childName: string;
  summary: string;
  children: ReactNode;
  reason: string;
  onReasonChange: (value: string) => void;
  reasonPlaceholder: string;
  reasonError?: string;
  busy: boolean;
  onApprove: () => void;
  onReject: () => void;
}>) {
  const [open, setOpen] = useState(false);
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50/60 sm:px-5"
      >
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900">
          {childName}
        </span>
        <span className="hidden shrink-0 truncate text-sm text-gray-500 sm:block">
          {summary}
        </span>
        <ChevronDown
          className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${open ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>
      {open && (
        <div className="border-t border-gray-100 px-4 pb-4 sm:px-5">
          {children}
          <div className="mt-4 space-y-2">
            <Input
              aria-label="Begründung"
              value={reason}
              placeholder={reasonPlaceholder}
              disabled={busy}
              onChange={(e) => onReasonChange(e.target.value)}
            />
            {reasonError && (
              <p className="text-xs text-[#CC2626]">{reasonError}</p>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={busy}
                onClick={onReject}
              >
                Ablehnen
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy}
                onClick={onApprove}
              >
                Freigeben
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * The calm "current → requested" panel shown inside an expanded review card,
 * matching the parent-side RequestDiffPanel surface (gray-50, muted uppercase
 * heading). Rows are passed as children so each queue keeps its own layout.
 */
export function ReviewDiffPanel({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <div className="mt-3 space-y-2 rounded-lg bg-gray-50 p-3">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Änderungen
      </p>
      {children}
    </div>
  );
}
