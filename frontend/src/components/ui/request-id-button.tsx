"use client";

import { useKitDefault } from "~/components/ui/kit-locale";
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
  const kitDefault = useKitDefault();

  const handleCopy = async () => {
    const copied = await copy(requestId);
    setFeedback(copied ? copiedLabel : copyFailedLabel);
  };

  return (
    <>
      {feedback ? <span role="status">{feedback}</span> : null}
      <Button
        type="button"
        variant="link_tone"
        size="compact"
        aria-label={copyLabel}
        // Sichtbar steht nur die Kennung. Auf einer übersetzten Fläche nennt
        // die Blase die Aktion mit dem übersetzten Namen; ActionHint liest
        // ihn dann nicht doppelt vor, weil er schon der Name ist.
        tooltip={
          kitDefault(
            "Kopiert die Kennung, damit Sie sie dem Support nennen können.",
          ) ?? copyLabel
        }
        onClick={() => void handleCopy()}
        // A request ID is a 36-character UUID: let it wrap inside the alert
        // or toast instead of running past its edge.
        className="max-w-full min-w-0 self-center text-left break-all whitespace-normal"
      >
        {label?.replace("{requestId}", requestId) ?? requestId}
      </Button>
    </>
  );
}
