"use client";

import { Check, Copy } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";

// Kein API-Fehler: der Browser hat das Kopieren verweigert. Der Link steht
// daneben und lässt sich von Hand markieren.
const COPY_FAILED =
  "Der Link konnte nicht kopiert werden. Bitte markieren und kopieren Sie ihn selbst.";

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
  const { showMessage } = useApiErrorDisplay();
  const Icon = copied ? Check : Copy;
  const accessibleLabel = copied ? "Link kopiert" : label;

  return (
    <Button
      type="button"
      variant={copied ? "surface_success" : "surface"}
      size="icon"
      className="shrink-0"
      onClick={(event) => {
        event.stopPropagation();
        void copy(url).then((ok) => {
          if (!ok) showMessage(COPY_FAILED);
        });
      }}
      aria-label={accessibleLabel}
      title={accessibleLabel}
    >
      <Icon
        className="h-4 w-4"
        aria-hidden="true"
      />
    </Button>
  );
}
