"use client";

// Controlled photo picker for the Datenverwaltung edit dialog. Compresses
// client-side and hands the Blob to the parent — no API calls here. The
// parent persists on Speichern.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Trash2, Upload } from "lucide-react";
import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";
import { compressAvatar } from "~/lib/image-utils";
import { createLogger } from "~/lib/logger";
import type { Student } from "~/lib/api";

const logger = createLogger({ component: "StudentPhotoSection" });

interface StudentPhotoSectionProps {
  student: Student;
  consentGiven: boolean;
  onConsentChange: (value: boolean) => void;
  // Non-null when the user has picked a file but not yet clicked Speichern.
  pendingPhotoBlob: Blob | null;
  // True when the user marked an existing server photo for deletion.
  pendingPhotoRemoved: boolean;
  // null clears a pending pick.
  onPickPhoto: (blob: Blob | null) => void;
  onMarkRemoved: () => void;
  onCancelRemove: () => void;
}

const ACCEPTED_TYPES = "image/jpeg,image/jpg,image/png,image/webp";

function formatGermanDateTime(iso?: string): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return d.toLocaleString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function StudentPhotoSection({
  student,
  consentGiven,
  onConsentChange,
  pendingPhotoBlob,
  pendingPhotoRemoved,
  onPickPhoto,
  onMarkRemoved,
  onCancelRemove,
}: StudentPhotoSectionProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);

  // Synchronous URL creation + revoke-on-change to avoid leaks.
  const blobUrl = useMemo(
    () => (pendingPhotoBlob ? URL.createObjectURL(pendingPhotoBlob) : null),
    [pendingPhotoBlob],
  );
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  }, [blobUrl]);

  const fullName = useMemo(() => {
    const first = student.first_name?.trim() ?? "";
    const last = student.second_name?.trim() ?? "";
    const combined = [first, last].filter(Boolean).join(" ");
    return combined || student.name || "Kind";
  }, [student.first_name, student.second_name, student.name]);

  const consentTimestamp = formatGermanDateTime(student.photo_consent_given_at);

  // Display priority: pending blob > marked-for-removal > server URL.
  const displayUrl = (() => {
    if (blobUrl) return blobUrl;
    if (pendingPhotoRemoved) return null;
    return student.photo_url ?? null;
  })();
  const hasAnyPhoto = displayUrl !== null;
  // pendingPhotoRemoved kept in the disjunction so the user can undo a
  // pending removal even when the avatar already shows initials.
  const showSecondaryAction =
    hasAnyPhoto || pendingPhotoBlob !== null || pendingPhotoRemoved;
  const hasPendingChange = pendingPhotoBlob !== null || pendingPhotoRemoved;

  const handleFilePicked = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      // Reset so re-picking the same file refires change.
      if (event.target) event.target.value = "";
      if (!file) return;

      setError(null);
      try {
        const compressed = await compressAvatar(file);
        onPickPhoto(compressed);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setError(message || "Foto konnte nicht verarbeitet werden");
        logger.error("student_photo_compress_failed", {
          student_id: student.id,
          error: message,
        });
      }
    },
    [onPickPhoto, student.id],
  );

  const handleRemoveClick = useCallback(() => {
    setError(null);
    if (pendingPhotoBlob !== null) {
      onPickPhoto(null); // discard pending pick
      return;
    }
    if (pendingPhotoRemoved) {
      onCancelRemove(); // undo pending removal
      return;
    }
    onMarkRemoved();
  }, [
    onCancelRemove,
    onMarkRemoved,
    onPickPhoto,
    pendingPhotoBlob,
    pendingPhotoRemoved,
  ]);

  return (
    <div className="bg-moto-blue/5 rounded-xl border border-gray-100 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className="text-moto-blue h-3.5 w-3.5 md:h-4 md:w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M3 9a2 2 0 012-2h.93a2 2 0 001.66-.9l.82-1.2A2 2 0 0110.07 4h3.86a2 2 0 011.66.9l.82 1.2a2 2 0 001.66.9H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
          />
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
        Foto
      </h3>

      <div className="flex items-center gap-4">
        <Avatar imageUrl={displayUrl} name={fullName} size="lg" />
        <div className="min-w-0 flex-1 text-xs text-gray-600">
          {pendingPhotoBlob ? (
            <p className="text-moto-blue font-medium">
              Foto ausgewählt — wird beim Speichern hochgeladen.
            </p>
          ) : pendingPhotoRemoved ? (
            <p className="text-moto-red-hover font-medium">
              Foto wird beim Speichern entfernt.
            </p>
          ) : displayUrl ? (
            <p>Aktuelles Foto ist hinterlegt.</p>
          ) : (
            <p>
              Kein Foto hinterlegt — Initialen werden als Platzhalter angezeigt.
            </p>
          )}
        </div>
      </div>

      <label
        htmlFor="photo-consent"
        className="moto-content-surface mt-4 flex cursor-pointer items-start gap-3 rounded-lg border p-3"
      >
        <input
          id="photo-consent"
          type="checkbox"
          checked={consentGiven}
          onChange={(e) => onConsentChange(e.target.checked)}
          className="text-moto-blue focus:ring-moto-blue mt-0.5 h-4 w-4 rounded border-gray-300"
          aria-label="Einwilligung der Eltern zur Speicherung eines Fotos liegt vor"
        />
        <div className="min-w-0 flex-1">
          <span className="block text-sm font-medium text-gray-900">
            Einwilligung der Eltern liegt vor
          </span>
          {consentGiven && consentTimestamp ? (
            <span className="mt-0.5 block text-xs text-gray-500">
              Erfasst am {consentTimestamp}
            </span>
          ) : (
            <span className="mt-0.5 block text-xs text-gray-500">
              Ein Foto kann erst nach Bestätigung der Einwilligung hochgeladen
              werden.
            </span>
          )}
        </div>
      </label>

      {consentGiven ? (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept={ACCEPTED_TYPES}
            onChange={handleFilePicked}
            className="hidden"
            tabIndex={-1}
          />
          <Button
            type="button"
            variant="primary"
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className="mr-1.5 h-4 w-4" aria-hidden />
            {hasAnyPhoto ? "Foto ersetzen" : "Foto auswählen"}
          </Button>
          {showSecondaryAction ? (
            <Button
              type="button"
              variant="secondary"
              onClick={handleRemoveClick}
            >
              <Trash2 className="mr-1.5 h-4 w-4" aria-hidden />
              {pendingPhotoBlob
                ? "Auswahl verwerfen"
                : pendingPhotoRemoved
                  ? "Entfernung rückgängig"
                  : "Foto entfernen"}
            </Button>
          ) : null}
          {hasPendingChange ? (
            <span className="text-xs text-gray-500">
              Klick auf <span className="font-medium">Speichern</span>, um die
              Änderung zu übernehmen.
            </span>
          ) : null}
        </div>
      ) : null}

      {error ? (
        <p className="text-moto-red-hover mt-2 text-xs" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
