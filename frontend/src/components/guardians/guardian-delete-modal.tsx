"use client";

import { ChevronRight, Loader2, Trash2, UserMinus } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";

type DeleteStep = "choose" | "confirm-unlink" | "confirm-full";

interface GuardianDeleteModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly guardianName: string;
  readonly isLoading?: boolean;
  /** Which step of the flow is shown. */
  readonly step?: DeleteStep;
  /**
   * Whether the current user may fully delete a guardian (admins only). Drives
   * both the second option card and whether the confirm step can step "back"
   * to a choice screen (non-admins have none).
   */
  readonly canFullDelete?: boolean;
  /**
   * Backend warning for the full-delete confirmation step — lists the children
   * (incl. siblings) that would lose this guardian. Shown on the "confirm-full"
   * step.
   */
  readonly fullDeleteWarning?: string | null;
  /**
   * True while the affected-children warning is still being fetched. The
   * confirm-full step opens instantly (so there is no dead "loading" gap) and
   * fills the warning in when it arrives; "Endgültig löschen" stays disabled
   * until then, so nobody confirms before seeing who is affected.
   */
  readonly isWarningLoading?: boolean;
  /** Choice: go to the per-child unlink confirmation. */
  readonly onSelectUnlink: () => void;
  /** Choice: start the full-delete flow (probes for the affected children). */
  readonly onSelectFullDelete: () => void;
  /** Confirm the per-child unlink. */
  readonly onConfirmUnlink: () => void;
  /** Confirm the full delete (force) once the warning has been shown. */
  readonly onConfirmFullDelete: () => void;
  /** Back from a confirmation step (→ choice for admins, → close otherwise). */
  readonly onBack: () => void;
}

// Selectable action row used on the choice step. The kit has no option-card
// primitive, so this is a small local component styled with kit tokens
// (rounded-xl, gray borders, brand red `#DC2626` = LOCATION_COLORS.DANGER).
function OptionCard({
  icon,
  title,
  subtitle,
  badge,
  danger = false,
  disabled = false,
  onClick,
}: {
  readonly icon: React.ReactNode;
  readonly title: string;
  readonly subtitle: string;
  readonly badge?: React.ReactNode;
  readonly danger?: boolean;
  readonly disabled?: boolean;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`group flex w-full items-center gap-3 rounded-xl border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
        danger
          ? "border-moto-red/30 hover:bg-moto-red/5"
          : "border-gray-200 hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      <span
        className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${
          danger
            ? "bg-moto-red/10 text-moto-red-strong"
            : "bg-gray-100 text-gray-600"
        }`}
      >
        {icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2">
          <span
            className={`text-sm font-semibold ${
              danger ? "text-moto-red-strong" : "text-gray-900"
            }`}
          >
            {title}
          </span>
          {badge}
        </span>
        <span className="mt-0.5 block text-xs text-gray-600">{subtitle}</span>
      </span>
      <ChevronRight className="h-4 w-4 shrink-0 text-gray-400 transition-transform group-hover:translate-x-0.5" />
    </button>
  );
}

const STEP_TITLES: Record<DeleteStep, (name: string) => string> = {
  choose: (name) => `${name} entfernen`,
  "confirm-unlink": () => "Von diesem Kind entfernen?",
  "confirm-full": () => "Komplett löschen?",
};

export function GuardianDeleteModal({
  isOpen,
  onClose,
  guardianName,
  isLoading = false,
  step = "choose",
  canFullDelete = false,
  fullDeleteWarning = null,
  isWarningLoading = false,
  onSelectUnlink,
  onSelectFullDelete,
  onConfirmUnlink,
  onConfirmFullDelete,
  onBack,
}: GuardianDeleteModalProps) {
  let footer: React.ReactNode;
  if (step === "choose") {
    footer = (
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onClose}
        disabled={isLoading}
      >
        Abbrechen
      </Button>
    );
  } else if (step === "confirm-unlink") {
    footer = (
      <>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onBack}
          disabled={isLoading}
        >
          {canFullDelete ? "Zurück" : "Abbrechen"}
        </Button>
        <Button
          type="button"
          variant="danger"
          size="sm"
          onClick={onConfirmUnlink}
          isLoading={isLoading}
          loadingText="Wird entfernt..."
        >
          Entfernen
        </Button>
      </>
    );
  } else {
    footer = (
      <>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onBack}
          disabled={isLoading}
        >
          Zurück
        </Button>
        <Button
          type="button"
          variant="danger"
          size="sm"
          onClick={onConfirmFullDelete}
          isLoading={isLoading}
          loadingText="Wird gelöscht..."
          disabled={isLoading || isWarningLoading}
        >
          Endgültig löschen
        </Button>
      </>
    );
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={STEP_TITLES[step](guardianName)}
      footer={footer}
    >
      {step === "choose" && (
        <div className="space-y-3">
          <p className="text-sm text-gray-700">Was möchten Sie tun?</p>

          <OptionCard
            icon={<UserMinus className="h-5 w-5" />}
            title="Nur von diesem Kind entfernen"
            subtitle="Bleibt für eventuelle Geschwister erhalten."
            disabled={isLoading}
            onClick={onSelectUnlink}
          />

          {/* Full delete — admins only. Reaches across every linked child.
              Kept visually neutral here on purpose; the destructive emphasis
              lives in the confirmation step, not in the choice. */}
          {canFullDelete && (
            <OptionCard
              icon={<Trash2 className="h-5 w-5" />}
              title="Vollständig löschen"
              subtitle="Entfernt die Person bei allen Kindern und löscht das Profil."
              badge={
                <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-gray-500 uppercase">
                  Admin
                </span>
              }
              disabled={isLoading}
              onClick={onSelectFullDelete}
            />
          )}
        </div>
      )}

      {step === "confirm-unlink" && (
        <p className="text-sm text-gray-700">
          Möchten Sie <strong>{guardianName}</strong> von diesem Kind entfernen?
          Für eventuelle Geschwister bleibt die Person erhalten.
        </p>
      )}

      {step === "confirm-full" && (
        <div className="space-y-3">
          {fullDeleteWarning ? (
            // Backend message — count-aware (singular vs. multiple children).
            // Brand red (LOCATION_COLORS.DANGER), same surface the kit
            // ConfirmDeleteModal uses for destructive warnings.
            <p className="bg-moto-red/10 text-moto-red-strong rounded-lg px-3 py-2 text-sm">
              {fullDeleteWarning}
            </p>
          ) : isWarningLoading ? (
            <p className="flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-2 text-sm text-gray-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              Betroffene Kinder werden geprüft…
            </p>
          ) : (
            <p className="text-sm text-gray-700">
              <strong>{guardianName}</strong> wird vollständig gelöscht.
            </p>
          )}
          <p className="text-moto-red text-sm font-medium">
            Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </div>
      )}
    </Modal>
  );
}
