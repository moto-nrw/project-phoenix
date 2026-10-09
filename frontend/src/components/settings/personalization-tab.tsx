"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import Image from "next/image";
import { ImageUp } from "lucide-react";
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
import { SectionCard } from "~/components/ui/section-card";

const logger = createLogger({ component: "PersonalizationTab" });

export function PersonalizationTab() {
  const { tenant } = useTenant();
  const router = useRouter();
  const { success: toastSuccess, error: toastError } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [currentImageUrl, setCurrentImageUrl] = useState<string | null>(
    tenant?.settings?.loginImageUrl ?? null,
  );
  const [canEdit, setCanEdit] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isUploading, setIsUploading] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isDragging, setIsDragging] = useState(false);

  // Laden: Fehler vor Ort mit Wiederholen, das Bild aus dem Schulkontext
  // bleibt stehen. Hochladen und Entfernen sind Aktionen ohne Formular: ein
  // Fehler kommt als Toast (#2517).
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
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
        toastError("Das Bild ist zu groß. Bitte wählen Sie ein Bild bis 2 MB.");
        return;
      }

      const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
      if (!allowedTypes.includes(file.type)) {
        toastError("Bitte wählen Sie ein Bild im Format JPG, PNG oder WebP.");
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
        if (fileInputRef.current) {
          fileInputRef.current.value = "";
        }
      }
    },
    [toastSuccess, toastError, router, showActionError],
  );

  const handleUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void processFile(file);
    },
    [processFile],
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

  const handleDragEnter = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragging(false);

      const file = e.dataTransfer.files[0];
      if (file) void processFile(file);
    },
    [processFile],
  );

  const handleZoneClick = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fileInputRef.current?.click();
    }
  }, []);

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
          <div className="flex h-[120px] items-center justify-center rounded-xl border border-gray-100 bg-gray-50">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-gray-600" />
          </div>
        ) : currentImageUrl ? (
          <div className="flex flex-col items-center gap-4 rounded-xl border border-gray-100 bg-gray-50 p-6">
            <Image
              src={loginImageSrc(currentImageUrl)}
              alt="Login-Bild"
              width={300}
              height={200}
              className="max-h-[200px] w-auto rounded object-contain"
              unoptimized
            />
            {canEdit && (
              <button
                type="button"
                onClick={handleDelete}
                disabled={isDeleting}
                className="border-moto-red/20 text-moto-red hover:bg-moto-red-soft inline-flex items-center gap-1.5 rounded-lg border bg-white px-3 py-1.5 text-sm font-medium shadow-sm transition-colors disabled:opacity-50"
              >
                {isDeleting ? "Wird entfernt…" : "Bild entfernen"}
              </button>
            )}
          </div>
        ) : loadError ? null : (
          // Ohne eigenes Bild zeigt die Login-Seite das moto-Logo. Die
          // Vorschau sagt das, statt die Karte leer zu lassen (#3893).
          <div className="moto-content-surface flex flex-col items-center gap-3 rounded-xl border p-6 shadow-sm">
            <MotoBrand />
            <p className="text-center text-sm text-gray-600">
              Noch kein eigenes Bild. Die Login-Seite zeigt das moto-Logo.
            </p>
          </div>
        )}

        {/* Upload dropzone — shown when editable */}
        {canEdit && (
          <div className={loadError && !currentImageUrl ? "" : "mt-4"}>
            {/* Hidden file input */}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              tabIndex={-1}
              onChange={handleUpload}
              disabled={isUploading}
              className="sr-only"
              aria-label="Bild auswählen"
            />

            <fieldset
              className={`relative m-0 rounded-xl border-2 border-dashed p-4 text-center transition-all duration-300 sm:p-8 ${
                isDragging
                  ? "border-moto-green bg-moto-green-soft"
                  : "border-gray-300 bg-gray-50 hover:border-gray-400"
              }`}
              onDragEnter={handleDragEnter}
              onDragLeave={handleDragLeave}
              onDragOver={handleDragOver}
              onDrop={handleDrop}
            >
              <legend className="sr-only">
                Bild-Upload-Bereich für Drag-and-Drop
              </legend>
              <button
                type="button"
                onClick={handleZoneClick}
                onKeyDown={handleKeyDown}
                aria-label="Bild hochladen — ziehen Sie eine Datei hierher oder klicken Sie zum Auswählen"
                className="focus:ring-moto-green absolute inset-0 z-10 cursor-pointer rounded-xl bg-transparent focus:ring-2 focus:ring-offset-2 focus:outline-none"
              />

              {isUploading ? (
                <div className="pointer-events-none flex flex-col items-center gap-3">
                  <div className="border-t-moto-green h-10 w-10 animate-spin rounded-full border-4 border-gray-300" />
                  <p className="text-sm text-gray-600">
                    Bild wird hochgeladen…
                  </p>
                </div>
              ) : (
                <div className="pointer-events-none flex flex-col items-center gap-3">
                  <ImageUp
                    className={`h-12 w-12 transition-colors ${isDragging ? "text-moto-green" : "text-gray-400"}`}
                    strokeWidth={1.5}
                    aria-hidden
                  />
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      {isDragging
                        ? "Bild hier ablegen…"
                        : currentImageUrl
                          ? "Neues Bild hierher ziehen"
                          : "Bild hierher ziehen"}
                    </p>
                    <p className="mt-0.5 text-xs text-gray-500">oder</p>
                  </div>
                  <span className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm text-white">
                    Bild auswählen
                  </span>
                  <p className="text-xs text-gray-400">
                    Max. 2 MB · JPG, PNG oder WebP · Empfohlen: ca. 900 × 650 px
                  </p>
                </div>
              )}
            </fieldset>
          </div>
        )}
      </SectionCard>
    </div>
  );
}
