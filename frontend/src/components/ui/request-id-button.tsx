"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";

interface RequestIdButtonProps {
  readonly requestId: string;
  /** Visible text, for example "Vorgangskennung: {requestId}". */
  readonly label?: string;
  readonly copyLabel: string;
  readonly copiedLabel: string;
  readonly copyFailedLabel: string;
}

/**
 * The request ID of a failed action as a button that copies it (ADR 0006), so
 * a person can pass it to support. Shared by the toast and the form alert.
 */
export function RequestIdButton({
  requestId,
  label,
  copyLabel,
  copiedLabel,
  copyFailedLabel,
}: RequestIdButtonProps) {
  const [feedback, setFeedback] = useState<string | null>(null);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(requestId);
      setFeedback(copiedLabel);
    } catch {
      setFeedback(copyFailedLabel);
    }
  };

  return (
    <>
      {feedback ? <span role="status">{feedback}</span> : null}
      <Button
        type="button"
        variant="ghost"
        size="compact"
        aria-label={copyLabel}
        onClick={() => void handleCopy()}
        // A request ID is a 36-character UUID: let it wrap inside the alert
        // or toast instead of running past its edge.
        className="h-auto min-h-8 max-w-full min-w-0 self-center py-1 text-left break-all whitespace-normal text-current underline underline-offset-2 hover:bg-black/5 hover:text-current"
      >
        {label?.replace("{requestId}", requestId) ?? requestId}
      </Button>
    </>
  );
}
