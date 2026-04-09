"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { useTenant } from "~/components/tenant/tenant-provider";
import { useToast } from "~/contexts/ToastContext";
import { sessionFetch } from "~/lib/session-cache";
import { loginImageSrc } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";

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

  const handleUpload = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

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

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-base font-medium text-gray-900">Login-Seite</h3>
        <p className="mt-1 text-sm text-gray-500">
          {canEdit
            ? "Laden Sie ein eigenes Bild hoch, das auf der Login-Seite Ihrer Einrichtung angezeigt wird. Ohne eigenes Bild wird das Standard-moto-Logo verwendet."
            : "Das aktuelle Bild wird auf der Login-Seite Ihrer Einrichtung angezeigt."}
        </p>
      </div>

      {/* Preview */}
      <div className="flex flex-col items-center gap-4 rounded-lg border border-dashed border-gray-300 bg-gray-50 p-6">
        {isLoading ? (
          <div className="flex h-[120px] w-[200px] items-center justify-center">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-gray-600" />
          </div>
        ) : currentImageUrl ? (
          <Image
            src={loginImageSrc(currentImageUrl)}
            alt="Login-Bild"
            width={300}
            height={200}
            className="max-h-[200px] w-auto rounded object-contain"
            unoptimized
          />
        ) : (
          <div className="flex h-[120px] w-[200px] items-center justify-center rounded bg-gray-200 text-sm text-gray-400">
            Kein Bild hochgeladen
          </div>
        )}

        {canEdit && (
          <p className="text-xs text-gray-400">
            Max. 2 MB · JPG, PNG oder WebP · Empfohlen: ca. 900 × 650 px
          </p>
        )}
      </div>

      {/* Actions — only shown for users with write permission */}
      {canEdit && (
        <div className="flex gap-3">
          <label
            className={`inline-flex cursor-pointer items-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 ${
              isUploading ? "pointer-events-none opacity-50" : ""
            }`}
          >
            {isUploading ? "Wird hochgeladen…" : "Bild hochladen"}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              className="hidden"
              onChange={handleUpload}
              disabled={isUploading}
            />
          </label>

          {currentImageUrl && (
            <button
              type="button"
              onClick={handleDelete}
              disabled={isDeleting}
              className="inline-flex items-center rounded-lg border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600 shadow-sm hover:bg-red-50 disabled:opacity-50"
            >
              {isDeleting ? "Wird entfernt…" : "Bild entfernen"}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
