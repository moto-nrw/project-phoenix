"use client";

import Image from "next/image";
import { ImageUp } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { SectionCard } from "~/components/ui/section-card";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  removeTraegerLogo,
  TRAEGER_LOGO_MAX_BYTES,
  type TraegerLogo,
  traegerLogoSrc,
  uploadTraegerLogo,
} from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerLogoCard" });

const LOGO_TYPES = ["image/png", "image/jpeg"];

/**
 * The logo of the Träger (#4653): shown, replaced by an upload or removed.
 * PNG or JPEG up to 1 MB; the backend checks the file once more.
 */
export function TraegerLogoCard({
  logo,
  canEdit,
  onChanged,
}: Readonly<{
  logo: TraegerLogo | null;
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}>) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [removing, setRemoving] = useState(false);
  const { show: showError, showMessage } = useApiErrorDisplay();
  const removeErrors = useApiFormError();
  const { success } = useToast();
  // The reload after a change stands above the card as a load error, never
  // as an error of the upload or the removal (#6652).
  const afterChange = useRefreshAfterWrite("die Anzeige des Logos", onChanged);

  const upload = async (file: File) => {
    if (!LOGO_TYPES.includes(file.type)) {
      showMessage("Bitte wählen Sie ein Bild im Format PNG oder JPG.");
      return;
    }
    if (file.size > TRAEGER_LOGO_MAX_BYTES) {
      showMessage("Das Logo ist zu groß. Bitte wählen Sie ein Bild bis 1 MB.");
      return;
    }
    setUploading(true);
    try {
      await uploadTraegerLogo(file);
    } catch (err) {
      logger.error("traeger_logo_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "das Logo" });
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
      return;
    }
    setUploading(false);
    if (inputRef.current) inputRef.current.value = "";
    success("Das Logo ist gespeichert.");
    await afterChange.refresh();
  };

  const remove = async () => {
    setRemoving(true);
    removeErrors.clear();
    try {
      await removeTraegerLogo();
    } catch (err) {
      logger.error("traeger_logo_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void removeErrors.show(err, { object: "das Entfernen des Logos" });
      setRemoving(false);
      return;
    }
    setRemoving(false);
    setConfirming(false);
    success("Das Logo ist entfernt.");
    await afterChange.refresh();
  };

  return (
    <SectionCard
      title="Logo"
      description="Das Logo steht oben links auf jedem Brief. PNG oder JPG, bis 1 MB und 2000 Pixel je Seite."
      actions={
        canEdit ? (
          <div className="flex flex-wrap gap-2">
            {logo ? (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                tooltip="Entfernt das Logo von allen Briefen. moto fragt vorher nach."
                onClick={() => setConfirming(true)}
              >
                Logo entfernen
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              size="md"
              isLoading={uploading}
              loadingText="Wird hochgeladen …"
              tooltip="Wählt ein Bild und setzt es sofort als Logo auf alle Briefe."
              onClick={() => inputRef.current?.click()}
            >
              <ImageUp className="mr-2 h-4 w-4" aria-hidden="true" />
              {logo ? "Anderes Logo hochladen" : "Logo hochladen"}
            </Button>
          </div>
        ) : null
      }
    >
      <LoadErrorAlert error={afterChange.error} className="mb-4" />
      <input
        ref={inputRef}
        type="file"
        accept="image/png,image/jpeg"
        className="sr-only"
        tabIndex={-1}
        aria-label="Logo-Datei auswählen"
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void upload(file);
        }}
      />
      {logo ? (
        <Image
          src={traegerLogoSrc(logo)}
          alt="Logo des Trägers"
          width={logo.width}
          height={logo.height}
          className="max-h-24 w-auto object-contain"
          unoptimized
        />
      ) : (
        <p className="text-sm text-gray-500">
          Es ist noch kein Logo hinterlegt.
        </p>
      )}
      <ConfirmDeleteModal
        isOpen={confirming}
        onClose={() => {
          setConfirming(false);
          removeErrors.clear();
        }}
        onConfirm={() => void remove()}
        title="Logo entfernen?"
        description="Briefe und Rechnungen haben danach kein Logo, bis Sie ein neues hochladen."
        confirmLabel="Entfernen"
        loadingLabel="Wird entfernt …"
        gate={{ mode: "twoStep", firstStepLabel: "Ja, entfernen" }}
        loading={removing}
        error={removeErrors.error}
      />
    </SectionCard>
  );
}
