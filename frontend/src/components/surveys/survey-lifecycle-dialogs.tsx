"use client";

/**
 * Die Rückfragen vor den Schritten einer Umfrage (#4781): Veröffentlichen,
 * Beenden, Erinnern, Löschen. Geteilt von Liste und Objektseite, damit
 * dieselbe Aktion überall dieselbe Rückfrage hat. Die API läuft erst im
 * `onConfirm` des Dialogs (BAUARTEN-SPEC Bauart 2 Regel 6).
 */

import { useLayoutEffect, useRef, useState } from "react";
import { BellRing, FileDown, Pencil, Send, Square, Trash2 } from "lucide-react";

import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  closeSurvey,
  deleteSurvey,
  publishSurvey,
  remindSurvey,
} from "~/lib/surveys-api";
import type { Survey } from "~/lib/surveys-api";
import {
  describeSurveyAudience,
  formatSurveyClosesAt,
  participantLabel,
} from "./survey-meta";
import { SURVEY_MIN_RESPONSES } from "./survey-questionnaire";

const logger = createLogger({ component: "SurveyLifecycle" });

interface DialogProps {
  readonly survey: Survey;
  readonly onClose: () => void;
  /** Läuft nach dem erfolgreichen Aufruf, vor dem Schließen. */
  readonly onDone: () => Promise<void> | void;
}

/** Ein Schritt mit Rückfrage: der Fehler bleibt im offenen Dialog. */
function useSurveyAction<T>(
  run: () => Promise<T>,
  {
    onDone,
    onClose,
    object,
    logEvent,
  }: {
    onDone: (result: T) => Promise<void> | void;
    onClose: () => void;
    object: string;
    logEvent: string;
  },
) {
  const [pending, setPending] = useState(false);
  const errors = useApiFormError();
  const latestConfirmRef = useRef<() => void>(() => undefined);

  const confirm = async () => {
    setPending(true);
    errors.clear();
    let result: T;
    try {
      result = await run();
    } catch (err) {
      logger.error(logEvent, {
        error: err instanceof Error ? err.message : String(err),
      });
      await errors.show(err, {
        object,
        retry: () => latestConfirmRef.current(),
      });
      setPending(false);
      return;
    }
    try {
      await onDone(result);
    } finally {
      setPending(false);
      onClose();
    }
  };
  useLayoutEffect(() => {
    latestConfirmRef.current = () => void confirm();
  });

  return { pending, error: errors.error, confirm };
}

export function PublishSurveyDialog({ survey, onClose, onDone }: DialogProps) {
  const { pending, error, confirm } = useSurveyAction(
    () => publishSurvey(survey.id),
    {
      onDone,
      onClose,
      object: "das Veröffentlichen der Umfrage",
      logEvent: "survey_publish_failed",
    },
  );

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="Umfrage veröffentlichen"
      confirmText="Jetzt veröffentlichen"
      cancelText="Abbrechen"
      isConfirmLoading={pending}
    >
      <div className="space-y-2 text-sm text-gray-700">
        <FormErrorAlert message={error} />
        <p>
          „{survey.title}“ geht an{" "}
          <span className="font-medium">{describeSurveyAudience(survey)}</span>.
          Teilnahme bis {formatSurveyClosesAt(survey.closes_at)}.
        </p>
        <p>
          {survey.anonymous
            ? `Anonym: Niemand sieht, wer was geantwortet hat. Die Auswertung erscheint nach dem Ende und ab ${SURVEY_MIN_RESPONSES} Antworten.`
            : "Nicht anonym: Bei jeder Antwort steht der Name."}
        </p>
        {survey.audience === "parents" && survey.send_email && (
          <p>Die Eltern bekommen zusätzlich eine E-Mail.</p>
        )}
        <p className="font-medium text-gray-900">
          Danach lässt sich nichts mehr ändern, auch nicht, ob die Umfrage
          anonym ist. Sie können sie nur noch beenden.
        </p>
      </div>
    </ConfirmationModal>
  );
}

export function CloseSurveyDialog({ survey, onClose, onDone }: DialogProps) {
  const { pending, error, confirm } = useSurveyAction(
    () => closeSurvey(survey.id),
    {
      onDone,
      onClose,
      object: "das Beenden der Umfrage",
      logEvent: "survey_close_failed",
    },
  );

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="Umfrage beenden"
      confirmText="Jetzt beenden"
      cancelText="Abbrechen"
      isConfirmLoading={pending}
    >
      <div className="space-y-2 text-sm text-gray-700">
        <FormErrorAlert message={error} />
        <p>
          Danach kann niemand mehr an „{survey.title}“ teilnehmen. Bisher:{" "}
          {participantLabel(survey.participant_count)}.
        </p>
        <p>
          Die Auswertung bleibt erhalten. Beenden lässt sich nicht rückgängig
          machen.
        </p>
      </div>
    </ConfirmationModal>
  );
}

export function RemindSurveyDialog({
  survey,
  onClose,
  onReminded,
}: Readonly<{
  survey: Survey;
  onClose: () => void;
  onReminded: (count: number) => Promise<void> | void;
}>) {
  const { pending, error, confirm } = useSurveyAction(
    () => remindSurvey(survey.id),
    {
      onDone: onReminded,
      onClose,
      object: "die Erinnerung",
      logEvent: "survey_remind_failed",
    },
  );

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="An die Umfrage erinnern"
      confirmText="Erinnerung senden"
      cancelText="Abbrechen"
      isConfirmLoading={pending}
    >
      <div className="space-y-2 text-sm text-gray-700">
        <FormErrorAlert message={error} />
        <p>
          Alle, die bei „{survey.title}“ noch nicht teilgenommen haben, bekommen
          eine Erinnerung.
        </p>
        <p>Sie erfahren nur die Zahl, nicht wer das ist.</p>
      </div>
    </ConfirmationModal>
  );
}

export function DeleteSurveyDialog({ survey, onClose, onDone }: DialogProps) {
  const { pending, error, confirm } = useSurveyAction(
    () => deleteSurvey(survey.id),
    {
      onDone,
      onClose,
      object: "das Löschen der Umfrage",
      logEvent: "survey_delete_failed",
    },
  );
  const withAnswers = survey.status !== "draft";

  return (
    <ConfirmDeleteModal
      isOpen
      title="Umfrage löschen"
      description={
        <>
          Möchten Sie die Umfrage
          <span className="font-medium text-gray-900"> „{survey.title}“ </span>
          wirklich löschen?{" "}
          {withAnswers
            ? `Alle Antworten und die Auswertung gehen verloren (${participantLabel(survey.participant_count)}). Das lässt sich nicht rückgängig machen.`
            : "Das lässt sich nicht rückgängig machen."}
        </>
      }
      gate={
        withAnswers
          ? {
              mode: "textConfirm",
              expected: "LÖSCHEN",
              inputId: "confirm-delete-survey",
              label: "Tippen Sie zur Bestätigung LÖSCHEN ein:",
              placeholder: "LÖSCHEN",
            }
          : { mode: "twoStep" }
      }
      onConfirm={confirm}
      onClose={onClose}
      loading={pending}
      error={error}
    />
  );
}

interface MenuHandlers {
  readonly onPublish: () => void;
  /** Nur in der Liste: die Objektseite ist die Ansicht selbst. */
  readonly onView?: () => void;
  readonly onEdit: () => void;
  readonly onRemind: () => void;
  readonly onCloseSurvey: () => void;
  /** Nur auf der Objektseite, wenn die Auswertung da ist. */
  readonly onExport?: () => void;
  readonly onDelete: () => void;
}

/**
 * Das Aktionsmenü einer Umfrage. Veröffentlichen eines Entwurfs steht oben;
 * Bearbeiten gibt es nur für Entwürfe. Eine veröffentlichte Umfrage lässt
 * sich nur noch erinnern, beenden und löschen.
 */
export function buildSurveyMenuItems(
  survey: Survey,
  handlers: MenuHandlers,
): OverflowMenuItem[] {
  const items: OverflowMenuItem[] = [];
  if (survey.status === "draft") {
    items.push({
      label: "Veröffentlichen",
      icon: <Send className="size-4" aria-hidden />,
      onClick: handlers.onPublish,
    });
  }
  if (handlers.onView) {
    items.push({
      label: survey.status === "draft" ? "Anzeigen" : "Auswertung anzeigen",
      icon: <MotoConceptIcon concept="reports" size={16} />,
      onClick: handlers.onView,
    });
  }
  if (survey.status === "draft") {
    items.push({
      label: "Bearbeiten",
      icon: <Pencil className="size-4" aria-hidden />,
      onClick: handlers.onEdit,
    });
  }
  if (survey.status === "open") {
    items.push(
      {
        label: "Erinnern",
        icon: <BellRing className="size-4" aria-hidden />,
        onClick: handlers.onRemind,
      },
      {
        label: "Beenden",
        icon: <Square className="size-4" aria-hidden />,
        onClick: handlers.onCloseSurvey,
      },
    );
  }
  if (handlers.onExport) {
    items.push({
      label: "Auswertung als PDF",
      icon: <FileDown className="size-4" aria-hidden />,
      onClick: handlers.onExport,
    });
  }
  items.push({
    label: "Löschen",
    icon: <Trash2 className="size-4" aria-hidden />,
    destructive: true,
    onClick: handlers.onDelete,
  });
  return items;
}
