"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { MotoBrand } from "~/components/auth/moto-brand";
import { useTenant } from "~/lib/tenant-context";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { sessionFetch } from "~/lib/session-cache";
import { loginImageSrc } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Button } from "~/components/ui/button";
import { Dropzone } from "~/components/ui/dropzone";
import { SpinnerIcon } from "~/components/ui/icons";
import { SectionCard } from "~/components/ui/section-card";
import { InsetPanel } from "~/components/ui/inset-panel";

const logger = createLogger({ component: "PersonalizationTab" });

export function PersonalizationTab() {
  const { tenant } = useTenant();
  const router = useRouter();
  const { success: toastSuccess } = useToast();

  const [currentImageUrl, setCurrentImageUrl] = useState<string | null>(
    tenant?.settings?.loginImageUrl ?? null,
  );
  const [canEdit, setCanEdit] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isUploading, setIsUploading] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  // Laden: Fehler vor Ort mit Wiederholen, das Bild aus dem Schulkontext
  // bleibt stehen. Hochladen und Entfernen sind Aktionen ohne Formular: ein
  // Fehler kommt als Toast (#2517).
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const { show: showActionError, showMessage } = useApiErrorDisplay();
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestUploadRef = useRef<(file: File) => void>(() => undefined);
  const latestDeleteRef = useRef<() => void>(() => undefined);

  const load = useCallback(async () => {
    setIsLoading(true);
    clear();
    try {
      const res = await sessionFetch("/api/settings/login-image");
      if (!res.ok) {
        throw await apiErrorFromResponse(
          res,
          `Login image fetch failed (${res.status})`,
        );
      }
      const json = (await res.json()) as {
        data?: {
          login_image_url?: string | null;
          can_edit?: boolean;
        };
      };
      setCurrentImageUrl(json?.data?.login_image_url ?? null);
      setCanEdit(json?.data?.can_edit ?? false);
    } catch (err) {
      logger.warn("login_image_fetch_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: "das Login-Bild",
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  }, [clear, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const processFile = useCallback(
    async (file: File) => {
      // Prüfung vor dem Hochladen. Es gibt kein Formular, an dem der Hinweis
      // stehen könnte; die Ablagefläche zeigt die Grenzen schon dauerhaft.
      const maxSize = 2 * 1024 * 1024;
      if (file.size > maxSize) {
        showMessage(
          "Das Bild ist zu groß. Bitte wählen Sie ein Bild bis 2 MB.",
        );
        return;
      }

      const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
      if (!allowedTypes.includes(file.type)) {
        showMessage("Bitte wählen Sie ein Bild im Format JPG, PNG oder WebP.");
        return;
      }

      setIsUploading(true);
      try {
        const formData = new FormData();
        formData.append("login_image", file);

        const response = await transportFetch("/api/settings/login-image", {
          method: "POST",
          body: formData,
        });

        if (!response.ok) {
          throw await apiErrorFromResponse(
            response,
            `Login image upload failed (${response.status})`,
          );
        }

        const result = (await response.json()) as {
          data: { login_image_url: string };
        };
        setCurrentImageUrl(result.data.login_image_url);
        toastSuccess("Das Login-Bild ist hochgeladen.");

        // Refresh server components so TenantProvider picks up the new settings
        router.refresh();
      } catch (error) {
        logger.error("login_image_upload_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showActionError(error, {
          object: "das Hochladen des Login-Bilds",
          retry: () => latestUploadRef.current(file),
        });
      } finally {
        setIsUploading(false);
      }
    },
    [toastSuccess, showMessage, router, showActionError],
  );

  const handleDelete = useCallback(async () => {
    setIsDeleting(true);
    try {
      const response = await transportFetch("/api/settings/login-image", {
        method: "DELETE",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `Login image delete failed (${response.status})`,
        );
      }

      setCurrentImageUrl(null);
      toastSuccess("Das Login-Bild ist entfernt.");

      // Refresh server components so TenantProvider picks up the removal
      router.refresh();
    } catch (error) {
      logger.error("login_image_delete_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      void showActionError(error, {
        object: "das Entfernen des Login-Bilds",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setIsDeleting(false);
    }
  }, [toastSuccess, router, showActionError]);

  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
    latestUploadRef.current = (file) => void processFile(file);
    latestDeleteRef.current = () => void handleDelete();
  });

  return (
    // Ein Gitter wächst nicht mit dem Seitenrumpf: Die Karte ist so hoch wie
    // ihr Inhalt, statt als leere weiße Fläche bis zur Unterkante zu reichen
    // (#3893).
    <div className="grid grid-cols-1 gap-6">
      <SectionCard
        headingLevel={3}
        title="Login-Seite"
        description={
          canEdit
            ? "Laden Sie ein eigenes Bild hoch, das auf der Login-Seite Ihrer Einrichtung angezeigt wird."
            : "So sieht die Login-Seite Ihrer Einrichtung oben aus. Ein neues Bild kann nur hochladen, wer Einstellungen ändern darf."
        }
      >
        {loadError ? (
          <LoadErrorAlert error={loadError} className="mb-4" />
        ) : null}
        {/* Current image preview */}
        {isLoading ? (
          <InsetPanel
            framed
            className="flex h-[120px] items-center justify-center"
          >
            <SpinnerIcon className="h-6 w-6 text-gray-500 motion-reduce:animate-none" />
          </InsetPanel>
        ) : currentImageUrl ? (
          <InsetPanel
            framed
            padding="md"
            className="flex flex-col items-center gap-4"
          >
            <Image
              src={loginImageSrc(currentImageUrl)}
              alt="Login-Bild"
              width={300}
              height={200}
              className="max-h-[200px] w-auto rounded object-contain"
              unoptimized
            />
            {canEdit && (
              <Button
                type="button"
                variant="outline_danger"
                size="compact"
                onClick={handleDelete}
                disabled={isDeleting}
              >
                {isDeleting ? "Wird entfernt…" : "Bild entfernen"}
              </Button>
            )}
          </InsetPanel>
        ) : loadError ? null : (
          // Ohne eigenes Bild zeigt die Login-Seite das moto-Logo. Die
          // Vorschau sagt das, statt die Karte leer zu lassen (#3893).
          <div className="moto-content-surface flex flex-col items-center gap-3 rounded-xl border p-6">
            <MotoBrand />
            <p className="text-center text-sm text-gray-600">
              Noch kein eigenes Bild. Die Login-Seite zeigt das moto-Logo.
            </p>
          </div>
        )}

        {/* Upload dropzone — shown when editable */}
        {canEdit && (
          <div className={loadError && !currentImageUrl ? "" : "mt-4"}>
            <Dropzone
              accept="image/jpeg,image/png,image/webp"
              legend="Bereich zum Hochladen des Login-Bilds"
              label={
                currentImageUrl
                  ? "Neues Bild hierher ziehen"
                  : "Bild hierher ziehen"
              }
              dragLabel="Bild hier ablegen…"
              buttonLabel="Bild auswählen"
              hint="Max. 2 MB · JPG, PNG oder WebP · Empfohlen: ca. 900 × 650 px"
              loading={isUploading}
              loadingLabel="Bild wird hochgeladen…"
              onFile={(file) => void processFile(file)}
            />
          </div>
        )}
      </SectionCard>
    </div>
  );
}
