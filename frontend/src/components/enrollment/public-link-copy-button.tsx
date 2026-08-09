"use client";

import { Check, Copy } from "lucide-react";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";

interface PublicLinkCopyButtonProps {
  readonly url: string;
  readonly componentId: string;
  readonly label?: string;
}

export function PublicLinkCopyButton({
  url,
  componentId,
  label = "Elternlink kopieren",
}: PublicLinkCopyButtonProps) {
  const { copied, copy } = useClipboardCopy(componentId, 2000);
  const Icon = copied ? Check : Copy;
  const accessibleLabel = copied ? "Link kopiert" : label;

  return (
    <button
      type="button"
      onClick={(event) => {
        event.stopPropagation();
        void copy(url);
      }}
      aria-label={accessibleLabel}
      title={accessibleLabel}
      className={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border bg-white shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        copied
          ? "border-moto-green/40 bg-moto-green/10 text-moto-green-strong"
          : "border-gray-200 text-gray-500 hover:bg-gray-50 hover:text-gray-900"
      }`}
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
    </button>
  );
}
