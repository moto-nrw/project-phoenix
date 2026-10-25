"use client";

import { useCallback, useState } from "react";

import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { fetchSurveyForm, submitSurveyAnswers } from "~/lib/surveys-api";
import type { SurveyToAnswer } from "~/lib/surveys-api";
import { SurveyAnswerPanel } from "./survey-questionnaire";

/**
 * Eine Umfrage an das Team beantworten (#4781). Der Fragebogen ist derselbe
 * wie im Elternportal; der Hinweis, ob die Umfrage anonym ist, steht vor der
 * ersten Frage.
 */
export function SurveyAnswerModal({
  survey,
  onClose,
  onSubmitted,
}: Readonly<{
  survey: SurveyToAnswer;
  onClose: () => void;
  onSubmitted: () => void;
}>) {
  const [busy, setBusy] = useState(false);
  const load = useCallback(() => fetchSurveyForm(survey.id), [survey.id]);
  const submit = useCallback(
    (body: Parameters<typeof submitSurveyAnswers>[1]) =>
      submitSurveyAnswers(survey.id, body),
    [survey.id],
  );

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={survey.title}
      isDismissDisabled={busy}
      isBackdropDismissDisabled
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      footer={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={onClose}
          disabled={busy}
        >
          Schließen
        </Button>
      }
    >
      <SurveyAnswerPanel
        load={load}
        submit={submit}
        respondent="staff"
        reloadKey={survey.id}
        onBusyChange={setBusy}
        onSubmitted={onSubmitted}
      />
    </Modal>
  );
}
