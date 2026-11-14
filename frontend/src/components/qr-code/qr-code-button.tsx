"use client";

import dynamic from "next/dynamic";
import { useState } from "react";
import { QrCode } from "lucide-react";
import { Button } from "~/components/ui/button";
import type { QrCodeDialogProps } from "./qr-code-dialog";

/**
 * The dialog with the QR library loads on the first click, so a page with a
 * link does not carry it for everyone who only copies.
 */
export const LazyQrCodeDialog = dynamic(
  () => import("./qr-code-dialog").then((module) => module.QrCodeDialog),
  { ssr: false },
);

type QrCodeTarget = Pick<
  QrCodeDialogProps,
  "url" | "title" | "fileName" | "brand"
>;

interface QrCodeButtonProps extends QrCodeTarget {
  /** `icon` for the tight row beside an icon copy button. */
  readonly appearance?: "labelled" | "icon";
  readonly className?: string;
}

/** Opens the QR code of a link. It stands beside the button that copies it. */
export function QrCodeButton({
  appearance = "labelled",
  className,
  ...target
}: QrCodeButtonProps) {
  const [open, setOpen] = useState(false);
  // Stays mounted after the first click so closing can animate.
  const [loaded, setLoaded] = useState(false);
  const show = () => {
    setLoaded(true);
    setOpen(true);
  };

  return (
    <>
      {appearance === "icon" ? (
        <Button
          type="button"
          variant="surface"
          size="icon"
          className={className ?? "shrink-0"}
          aria-label="QR-Code anzeigen"
          tooltip="Zeigt den Link als QR-Code zum Scannen oder Speichern."
          onClick={(event) => {
            event.stopPropagation();
            show();
          }}
        >
          <QrCode className="h-4 w-4" aria-hidden="true" />
        </Button>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="md"
          className={className ?? "inline-flex shrink-0 items-center gap-2"}
          onClick={show}
          tooltip="Zeigt den Link als QR-Code zum Scannen oder Speichern."
        >
          <QrCode className="h-4 w-4" aria-hidden="true" />
          QR-Code
        </Button>
      )}
      {loaded ? (
        <LazyQrCodeDialog
          {...target}
          isOpen={open}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}
