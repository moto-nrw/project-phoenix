"use client";

import { Loader2 } from "lucide-react";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";

export type GuardianDeleteScope = "unlink" | "full";

interface GuardianDeleteModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly guardianName: string;
  readonly isLoading?: boolean;
  /**
   * Whether the current user may fully delete a guardian (admins only). Only
   * then does the dialog offer the scope choice; everyone else gets the
   * per-child unlink straight away.
   */
  readonly canFullDelete?: boolean;
  /** Controlled scope choice (admins). `null` until the user has picked. */
  readonly scope?: GuardianDeleteScope | null;
  readonly onScopeChange?: (scope: GuardianDeleteScope) => void;
  /**
   * Backend warning for the full delete — lists the children (incl. siblings)
   * that would lose this guardian. Shown once "Vollständig löschen" is chosen.
   */
  readonly fullDeleteWarning?: string | null;
  /**
   * True while the affected-children warning is still being fetched. The
   * confirm button stays disabled until it arrives, so nobody confirms before
   * seeing who is affected.
   */
  readonly isWarningLoading?: boolean;
  /** Confirm the per-child unlink. */
  readonly onConfirmUnlink: () => void;
  /** Confirm the full delete (force) once the warning has been shown. */
  readonly onConfirmFullDelete: () => void;
}

// Löschen einer Erziehungsberechtigten-Verknüpfung (#3110): ein Dialog auf
// ConfirmDeleteModal. Die Wahl „nur von diesem Kind“ / „vollständig löschen“
// liegt als Scope-Slot im Dialog; das vollständige Löschen ist
// unwiderruflicher Datenverlust und verlangt deshalb die Namenseingabe.
export function GuardianDeleteModal({
  isOpen,
  onClose,
  guardianName,
  isLoading = false,
  canFullDelete = false,
  scope = null,
  onScopeChange,
  fullDeleteWarning = null,
  isWarningLoading = false,
  onConfirmUnlink,
  onConfirmFullDelete,
}: GuardianDeleteModalProps) {
  const fullDelete = canFullDelete && scope === "full";

  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      title={`${guardianName} entfernen`}
      description={
        canFullDelete ? (
          <p>
            <strong>{guardianName}</strong> ist mit diesem Kind verknüpft.
          </p>
        ) : (
          <p>
            <strong>{guardianName}</strong> wird von diesem Kind entfernt. Für
            eventuelle Geschwister bleibt die Person erhalten.
          </p>
        )
      }
      scope={
        canFullDelete
          ? {
              label: "Was möchten Sie tun?",
              name: "guardian-delete-scope",
              value: scope,
              onChange: (value) =>
                onScopeChange?.(value === "full" ? "full" : "unlink"),
              options: [
                {
                  value: "unlink",
                  label: "Nur von diesem Kind entfernen",
                  description: "Bleibt für eventuelle Geschwister erhalten.",
                },
                {
                  value: "full",
                  label: "Vollständig löschen",
                  description:
                    "Entfernt die Person bei allen Kindern und löscht das Profil.",
                },
              ],
            }
          : undefined
      }
      warningSlot={
        fullDelete ? (
          <div className="space-y-2">
            {fullDeleteWarning ? (
              // Backend message — count-aware (singular vs. multiple children).
              <p className="bg-moto-red/10 text-moto-red-strong rounded-lg px-3 py-2 text-sm">
                {fullDeleteWarning}
              </p>
            ) : isWarningLoading ? (
              <p className="flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-2 text-sm text-gray-500">
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Betroffene Kinder werden geprüft…
              </p>
            ) : null}
            <p className="text-moto-red text-sm font-medium">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          </div>
        ) : undefined
      }
      gate={
        fullDelete
          ? {
              mode: "textConfirm",
              expected: guardianName,
              inputId: "guardian-full-delete-confirm",
              label: "Geben Sie zur Bestätigung den Namen ein:",
              placeholder: guardianName,
              preview: guardianName,
            }
          : { mode: "twoStep", firstStepLabel: "Entfernen" }
      }
      confirmDisabled={fullDelete && (isWarningLoading || !fullDeleteWarning)}
      confirmLabel={
        fullDelete ? "Endgültig löschen" : "Von diesem Kind entfernen"
      }
      loadingLabel={fullDelete ? "Wird gelöscht…" : "Wird entfernt…"}
      onConfirm={fullDelete ? onConfirmFullDelete : onConfirmUnlink}
      onClose={onClose}
      loading={isLoading}
      error=""
    />
  );
}
