"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { ImageUp } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTenant } from "~/lib/tenant-context";
import { useToast } from "~/contexts/ToastContext";
import { sessionFetch } from "~/lib/session-cache";
import { loginImageSrc } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";
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

  // Fetch current login image + edit permission on mount.
  // On failure, keep the existing preview from tenant context instead of clearing it.
  useEffect(() => {
    sessionFetch("/api/settings/login-image")
      .then((res) => {
        if (!res.ok) {
          logger.warn("login_image_fetch_non_ok", { status: res.status });
          return;
        }
        return res.json().then(
          (json: {
            data?: {
              login_image_url?: string | null;
              can_edit?: boolean;
            };
          }) => {
            setCurrentImageUrl(json?.data?.login_image_url ?? null);
            setCanEdit(json?.data?.can_edit ?? false);
          },
        );
      })
      .catch((err) => {
        logger.warn("login_image_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => setIsLoading(false));
  }, []);

  const processFile = useCallback(
    async (file: File) => {
      const maxSize = 2 * 1024 * 1024;
      if (file.size > maxSize) {
        toastError("Datei ist zu groß (max. 2 MB)");
        return;
      }

      const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
      if (!allowedTypes.includes(file.type)) {
        toastError("Nur JPG, PNG oder WebP erlaubt");
        return;
      }

      setIsUploading(true);
      try {
        const formData = new FormData();
        formData.append("login_image", file);

        const response = await fetch("/api/settings/login-image", {
          method: "POST",
          body: formData,
        });

        if (!response.ok) {
          const errorBody = await response.text();
          throw new Error(`Upload failed: ${response.status} — ${errorBody}`);
        }

        const result = (await response.json()) as {
          data: { login_image_url: string };
        };
        setCurrentImageUrl(result.data.login_image_url);
        toastSuccess("Login-Bild erfolgreich hochgeladen");

        // Refresh server components so TenantProvider picks up the new settings
        router.refresh();
      } catch (error) {
        logger.error("login_image_upload_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        toastError("Fehler beim Hochladen des Bildes");
      } finally {
        setIsUploading(false);
        if (fileInputRef.current) {
          fileInputRef.current.value = "";
        }
      }
    },
    [toastSuccess, toastError, router],
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
      const response = await fetch("/api/settings/login-image", {
        method: "DELETE",
      });

      if (!response.ok) {
        throw new Error(`Delete failed: ${response.status}`);
      }

      setCurrentImageUrl(null);
      toastSuccess("Login-Bild erfolgreich entfernt");

      // Refresh server components so TenantProvider picks up the removal
      router.refresh();
    } catch (error) {
      logger.error("login_image_delete_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      toastError("Fehler beim Entfernen des Bildes");
    } finally {
      setIsDeleting(false);
    }
  }, [toastSuccess, toastError, router]);

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
    <div className="space-y-6">
      <SectionCard
        headingLevel={3}
        title="Login-Seite"
        description={
          canEdit
            ? "Laden Sie ein eigenes Bild hoch, das auf der Login-Seite Ihrer Einrichtung angezeigt wird."
            : "Das aktuelle Bild wird auf der Login-Seite Ihrer Einrichtung angezeigt."
        }
      >
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
        ) : null}

        {/* Upload dropzone — shown when editable */}
        {canEdit && (
          <div className={currentImageUrl ? "mt-4" : ""}>
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
