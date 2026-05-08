"use client";

// Datenverwaltung edit-dialog photo block.
//
// Visible only when the tenant has `operations.student_photos_enabled = true`
// (gated via useStudentPhotosEnabled). Renders nothing otherwise — the rest
// of the form stays unchanged for schools that haven't opted in.
//
// IMPORTANT: this section is a *controlled component*. It owns no API calls
// and no server-side mutation state. The user picks a file → we compress it
// client-side → we hand the resulting Blob to the parent (`onPickPhoto`).
// The parent (`student-stammdaten-tab`) holds the pending photo + the
// pending-removal flag and ships them to the backend only when the user
// clicks "Speichern". This way the form behaves like every other field:
// nothing leaves the browser until the user explicitly commits.

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
  /** Whether the parental-consent box is currently checked in the form draft. */
  consentGiven: boolean;
  /** Toggles the consent flag in the form draft (parent persists via onSave). */
  onConsentChange: (value: boolean) => void;
  /**
   * Pending photo blob from the parent's form state. Non-null when the
   * user has picked a file but not yet clicked Speichern. The preview
   * avatar uses a blob: URL derived from this; on submit the parent
   * uploads it.
   */
  pendingPhotoBlob: Blob | null;
  /**
   * True when the user clicked "Foto entfernen" on a server-side photo and
   * has not yet clicked Speichern. The parent will issue DELETE on submit.
   */
  pendingPhotoRemoved: boolean;
  /**
   * Called when the user picks a new file (after client-side compression)
   * or, with `null`, when the user un-picks a pending file (e.g. by
   * clicking "Verwerfen" on a not-yet-saved selection).
   */
  onPickPhoto: (blob: Blob | null) => void;
  /** Called when the user clicks "Foto entfernen" on a server-side photo. */
  onMarkRemoved: () => void;
  /**
   * Called when the user reverses a not-yet-saved removal — they ticked
   * "Foto entfernen" on a server-side photo, then changed their mind
   * before clicking Speichern. The parent should clear pendingPhotoRemoved
   * so the existing server photo stays in place. Without this control the
   * only escapes from a mistaken click are leaving the form (losing
   * unsaved field edits) or uploading a replacement photo — neither
   * matches the user's intent of "actually no, keep the photo".
   */
  onCancelRemove: () => void;
}

const ACCEPTED_TYPES = "image/jpeg,image/jpg,image/png,image/webp";

/** Format an ISO timestamp to a German short datetime, e.g. "06.05.2026, 14:32". */
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

  // Lifecycle for the blob: URL previewing the freshly-picked file.
  // useMemo creates the URL synchronously so the avatar paints on the
  // same frame as the pick; the cleanup effect revokes it whenever the
  // blob changes or the component unmounts so we don't leak memory.
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

  // Display priority: locally-picked blob > marked-for-removal > server URL.
  // The fall-through to initials happens automatically inside <Avatar>.
  const displayUrl = (() => {
    if (blobUrl) return blobUrl;
    if (pendingPhotoRemoved) return null;
    return student.photo_url ?? null;
  })();
  // hasAnyPhoto drives the upload-button copy ("Foto auswählen" vs
  // "Foto ersetzen"). After marking the server photo for removal, the
  // user's mental model is "no photo any more" — flip the copy to match.
  const hasAnyPhoto = displayUrl !== null;
  // The secondary action (verwerfen / rückgängig / entfernen) must stay
  // visible whenever there is a pending change to undo OR a server photo
  // that could still be marked for removal. Specifically: once
  // pendingPhotoRemoved is true, hasAnyPhoto becomes false (the avatar
  // shows initials), so without this third clause the only control that
  // could undo the pending removal would disappear — leaving the user
  // stuck. See [P3] on the photo-feature review.
  const showSecondaryAction =
    hasAnyPhoto || pendingPhotoBlob !== null || pendingPhotoRemoved;
  const hasPendingChange = pendingPhotoBlob !== null || pendingPhotoRemoved;

  const handleFilePicked = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      // Always reset the input value so the user can re-pick the same file
      // after a failed compression (the change event won't refire on an
      // identical selection otherwise).
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
      // User changes their mind about a not-yet-saved selection.
      onPickPhoto(null);
      return;
    }
    if (pendingPhotoRemoved) {
      // Second click after marking for removal — undo the pending
      // deletion before Speichern fires.
      onCancelRemove();
      return;
    }
    // Mark the server-side photo for deletion on next Speichern.
    onMarkRemoved();
  }, [
    onCancelRemove,
    onMarkRemoved,
    onPickPhoto,
    pendingPhotoBlob,
    pendingPhotoRemoved,
  ]);

  return (
    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
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
            <p className="font-medium text-[#5080D8]">
              Foto ausgewählt — wird beim Speichern hochgeladen.
            </p>
          ) : pendingPhotoRemoved ? (
            <p className="font-medium text-[#B82A29]">
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
        className="mt-4 flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 bg-white p-3"
      >
        <input
          id="photo-consent"
          type="checkbox"
          checked={consentGiven}
          onChange={(e) => onConsentChange(e.target.checked)}
          className="mt-0.5 h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
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

      {/* Upload + delete enable as soon as the consent checkbox is on. The
          actual upload waits for Speichern — these buttons only update
          local form state. */}
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
        <p className="mt-2 text-xs text-red-700" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
