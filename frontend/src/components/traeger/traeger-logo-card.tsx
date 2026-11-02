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
      success("Das Logo ist gespeichert.");
      await onChanged();
    } catch (err) {
      logger.error("traeger_logo_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "das Logo" });
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  const remove = async () => {
    setRemoving(true);
    removeErrors.clear();
    try {
      await removeTraegerLogo();
      setConfirming(false);
      success("Das Logo ist entfernt.");
      await onChanged();
    } catch (err) {
      logger.error("traeger_logo_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void removeErrors.show(err, { object: "das Entfernen des Logos" });
    } finally {
      setRemoving(false);
    }
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
              onClick={() => inputRef.current?.click()}
            >
              <ImageUp className="mr-2 h-4 w-4" aria-hidden="true" />
              {logo ? "Anderes Logo hochladen" : "Logo hochladen"}
            </Button>
          </div>
        ) : null
      }
    >
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
