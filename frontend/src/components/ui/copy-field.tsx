"use client";

import type { ReactNode } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";
import { cn } from "~/lib/utils";

interface CopyFieldProps {
  readonly value: string;
  /** For technical values such as keys and links. */
  readonly monospace?: boolean;
  /** Text of the button. Name the object where one screen shows several. */
  readonly copyLabel?: string;
  /** What the copy button does, in one sentence (`Button` `tooltip`). */
  readonly copyHint?: string;
  /**
   * The browser refused the clipboard. The value stays selectable; the caller
   * tells the person to copy it by hand.
   */
  readonly onCopyFailed?: () => void;
  /**
   * A second way to hand the value on, right beside „Kopieren“: the QR code
   * of a link (`QrCodeButton`). On a phone the buttons move under the value.
   */
  readonly action?: ReactNode;
  readonly className?: string;
}

/**
 * A value to read and copy, in full (#5599). It wraps instead of truncating:
 * a one-time key or link has to stay checkable on the screen that shows it.
 */
export function CopyField({
  value,
  monospace = false,
  copyLabel = "Kopieren",
  copyHint,
  onCopyFailed,
  action,
  className,
}: CopyFieldProps) {
  const { copied, copy } = useClipboardCopy("CopyField", 2000);
  const Icon = copied ? Check : Copy;

  return (
    <div
      className={cn(
        "flex min-w-0 gap-2",
        action ? "flex-col sm:flex-row sm:items-center" : "items-center",
        className,
      )}
    >
      <code
        className={cn(
          "min-w-0 flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 break-all text-gray-700 select-all",
          monospace ? "font-mono text-xs" : "font-sans text-sm",
        )}
      >
        {value}
      </code>
      <div className="flex shrink-0 flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="md"
          className="inline-flex shrink-0 items-center gap-2"
          tooltip={copyHint}
          onClick={() => {
            void copy(value).then((ok) => {
              if (!ok) onCopyFailed?.();
            });
          }}
        >
          <Icon
            className={cn("h-4 w-4", copied && "text-moto-green-strong")}
            aria-hidden="true"
          />
          {copied ? "Kopiert" : copyLabel}
        </Button>
        {action}
      </div>
      {/* The button's new name alone is not announced. */}
      <output className="sr-only">
        {copied ? "In die Zwischenablage kopiert" : ""}
      </output>
    </div>
  );
}
