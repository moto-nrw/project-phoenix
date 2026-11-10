"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";

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
  const { copy } = useClipboardCopy("RequestIdButton");

  const handleCopy = async () => {
    const copied = await copy(requestId);
    setFeedback(copied ? copiedLabel : copyFailedLabel);
  };

  const text = label?.replace("{requestId}", requestId) ?? requestId;
  const at = text.lastIndexOf(requestId);

  return (
    <>
      {feedback ? <span role="status">{feedback}</span> : null}
      <Button
        type="button"
        variant="link_tone"
        size="compact"
        aria-label={copyLabel}
        onClick={() => void handleCopy()}
        // A request ID is a 36-character UUID: let it wrap inside the alert
        // or toast instead of running past its edge.
        className="max-w-full min-w-0 self-center text-left break-all whitespace-normal"
      >
        {at > 0 ? (
          // One flow inside the flex button: on a phone the label takes its
          // own line, so the ID below it stays whole instead of leaving its
          // last characters alone.
          <span className="min-w-0">
            <span className="max-sm:block">{text.slice(0, at).trimEnd()}</span>{" "}
            <span>{requestId}</span>
            {text.slice(at + requestId.length)}
          </span>
        ) : (
          text
        )}
      </Button>
    </>
  );
}
