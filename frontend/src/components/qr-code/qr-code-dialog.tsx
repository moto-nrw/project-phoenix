"use client";

import { useMemo, useState } from "react";
import { Download } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Dropzone } from "~/components/ui/dropzone";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Modal } from "~/components/ui/modal";
import { SectionLabel } from "~/components/ui/section-label";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { brandedQrLayout, brandedQrSvg } from "~/lib/qr-code/branded-qr";
import { brandedQrPng } from "~/lib/qr-code/qr-image";
import { MOTO_QR_LOGO, QrCodeImage } from "./branded-qr-code";
import { useQrLogo, type QrLogoSource } from "./use-qr-logo";

const logger = createLogger({ component: "QrCodeDialog" });

const IMAGE_TYPES = ["image/png", "image/jpeg", "image/webp"];
const IMAGE_MAX_BYTES = 5 * 1024 * 1024;
/** 2048 px prints sharply up to about 17 cm at 300 dpi. */
const PNG_SIZE = 2048;

/** The logo a school or Träger has stored in moto. */
export interface QrBrandLogo {
  /** Same-origin address of the picture. */
  readonly src: string;
  /** Who it belongs to, as the choice names it: „Schule“, „Träger“. */
  readonly label: string;
}

export interface QrCodeDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** The link the code leads to. */
  readonly url: string;
  /** Title of the dialog, such as „QR-Code für die Anmeldung“. */
  readonly title: string;
  /** Name of the downloaded file, without ending. */
  readonly fileName: string;
  /** Stored logo; the default when it exists, otherwise the moto logo. */
  readonly brandLogo?: QrBrandLogo;
}

type LogoChoice = "brand" | "moto" | "custom";

/**
 * Shows a link as a QR code in the moto look and saves it as a picture. The
 * logo in the middle is the stored one, the moto logo or a chosen picture;
 * a chosen picture stays in the browser.
 */
export function QrCodeDialog({
  isOpen,
  onClose,
  url,
  title,
  fileName,
  brandLogo,
}: QrCodeDialogProps) {
  const [choice, setChoice] = useState<LogoChoice | null>(null);
  const [customImage, setCustomImage] = useState<File | null>(null);
  const [removeBackground, setRemoveBackground] = useState(true);
  const [saving, setSaving] = useState<"png" | "svg" | null>(null);
  const errors = useApiFormError();

  // The stored logo loads once, to learn whether there is one at all.
  const brand = useQrLogo(isOpen ? (brandLogo?.src ?? null) : null, true);
  const brandAvailable = brandLogo !== undefined && brand.status === "ready";
  const brandPending = brandLogo !== undefined && brand.status === "loading";
  const activeChoice: LogoChoice =
    choice ?? (brandAvailable ? "brand" : "moto");

  let source: QrLogoSource | null = MOTO_QR_LOGO;
  if (activeChoice === "brand") source = brandLogo?.src ?? null;
  if (activeChoice === "custom") source = customImage;
  // The moto logo is already cut out; the switch is for the other two.
  const prepared = useQrLogo(
    isOpen && !brandPending ? source : null,
    activeChoice === "moto" ? true : removeBackground,
  );
  const logo = prepared.logo;
  const waitingForLogo =
    brandPending || (source !== null && prepared.status === "loading");

  const layout = useMemo(
    () => brandedQrLayout(url, logo?.aspect),
    [url, logo?.aspect],
  );
  const svg = useMemo(
    () => brandedQrSvg(layout, { label: title, logoDataUrl: logo?.dataUrl }),
    [layout, title, logo?.dataUrl],
  );

  const choices: SegmentedControlItem<LogoChoice>[] = [
    ...(brandAvailable && brandLogo
      ? [{ value: "brand" as const, label: brandLogo.label }]
      : []),
    { value: "moto", label: "moto" },
    { value: "custom", label: "Eigenes Bild" },
  ];

  const chooseImage = (file: File) => {
    errors.clear();
    if (!IMAGE_TYPES.includes(file.type)) {
      errors.invalid("Bitte wählen Sie ein Bild im Format PNG, JPG oder WebP.");
      return;
    }
    if (file.size > IMAGE_MAX_BYTES) {
      errors.invalid(
        "Das Bild ist zu groß. Bitte wählen Sie ein Bild bis 5 MB.",
      );
      return;
    }
    setCustomImage(file);
    setRemoveBackground(true);
  };

  const save = async (format: "png" | "svg") => {
    errors.clear();
    setSaving(format);
    try {
      const blob =
        format === "png"
          ? await brandedQrPng(layout, {
              logoDataUrl: logo?.dataUrl,
              pixelSize: PNG_SIZE,
            })
          : new Blob([svg], { type: "image/svg+xml" });
      downloadBlob(blob, `${fileName}.${format}`);
    } catch (err) {
      logger.error("qr_code_download_failed", {
        format,
        error: err instanceof Error ? err.message : String(err),
      });
      errors.invalid(
        "Der QR-Code konnte nicht gespeichert werden. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(null);
    }
  };

  const unreadable =
    activeChoice === "custom" &&
    customImage !== null &&
    prepared.status === "failed";
  const showBackgroundSwitch =
    activeChoice !== "moto" && logo?.hasPlainBackground === true;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      description="Eltern öffnen den Link, wenn sie den Code mit der Handy-Kamera scannen."
      size="md"
      mobileSheet
      footer={
        <>
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Schließen
          </Button>
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={waitingForLogo || saving !== null}
            isLoading={saving === "svg"}
            loadingText="Wird gespeichert…"
            onClick={() => void save("svg")}
          >
            Druckdatei (SVG)
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={waitingForLogo || saving !== null}
            isLoading={saving === "png"}
            loadingText="Wird gespeichert…"
            onClick={() => void save("png")}
          >
            <Download className="mr-2 h-4 w-4" aria-hidden="true" />
            Bild speichern
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <InsetPanel padding="md" className="flex flex-col items-center gap-3">
          <div className="moto-content-surface aspect-square w-full max-w-60 rounded-xl border p-3">
            {waitingForLogo ? (
              <Skeleton shape="inner" className="h-full w-full" />
            ) : (
              <QrCodeImage url={url} logo={logo} />
            )}
          </div>
          <p className="max-w-full text-center text-xs break-all text-gray-500">
            {url}
          </p>
        </InsetPanel>

        <div className="space-y-4">
          <div className="space-y-2">
            <SectionLabel id="qr-logo-choice">Logo in der Mitte</SectionLabel>
            <SegmentedControl
              items={choices}
              value={activeChoice}
              onChange={(next) => {
                errors.clear();
                setChoice(next);
              }}
              fullWidth
              ariaLabel="Logo in der Mitte"
            />
          </div>

          {activeChoice === "custom" ? (
            <Dropzone
              accept={IMAGE_TYPES.join(",")}
              legend="Bereich zum Auswählen des Bilds"
              label={customImage ? customImage.name : "Bild hierher ziehen"}
              dragLabel="Bild hier loslassen"
              buttonLabel={
                customImage ? "Anderes Bild wählen" : "Bild auswählen"
              }
              hint="PNG, JPG oder WebP, bis 5 MB. Das Bild bleibt auf Ihrem Gerät."
              onFile={chooseImage}
            />
          ) : null}

          {showBackgroundSwitch ? (
            <SwitchRow
              label="Hintergrund entfernen"
              description="Nimmt die einfarbige Fläche hinter dem Logo weg."
              checked={removeBackground}
              onCheckedChange={setRemoveBackground}
            />
          ) : null}

          <FormErrorAlert
            message={
              errors.error ??
              (unreadable
                ? "Das Bild konnte nicht gelesen werden. Bitte wählen Sie ein anderes Bild."
                : null)
            }
          />
        </div>
      </div>
    </Modal>
  );
}
