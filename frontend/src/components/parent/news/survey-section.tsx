"use client";

/**
 * Eine Umfrage mit Fragebogen im Elternbrief-Dialog (#4781). Eltern antworten
 * einmal je Elternkonto, nicht je Kind. Ob die Umfrage anonym ist, steht vor
 * der ersten Frage.
 */

import { useCallback } from "react";

import { SurveyAnswerPanel } from "~/components/surveys/survey-questionnaire";
import {
  fetchParentSurveyForm,
  submitParentSurveyAnswers,
  type ParentAnnouncement,
  type ParentAnnouncementSurvey,
} from "~/lib/parent-api";
import type { SurveyAnswersRequest } from "~/lib/survey-form";

export function isSurveyItem(
  item: ParentAnnouncement,
): item is ParentAnnouncement & { survey: ParentAnnouncementSurvey } {
  return item.system_kind === "survey" && item.survey !== undefined;
}

/** Noch offen und von diesem Elternkonto noch nicht beantwortet. */
export function isOpenSurvey(item: ParentAnnouncement): boolean {
  if (!isSurveyItem(item)) return false;
  return (
    !item.survey.participated && new Date(item.survey.closes_at) > new Date()
  );
}

export function ParentSurveySection({
  item,
  onUpdated,
  onBusyChange,
}: Readonly<{
  item: ParentAnnouncement & { survey: ParentAnnouncementSurvey };
  onUpdated: (id: string, patch: Partial<ParentAnnouncement>) => void;
  onBusyChange: (busy: boolean) => void;
}>) {
  const announcementId = item.id;
  const survey = item.survey;
  const load = useCallback(
    () => fetchParentSurveyForm(announcementId),
    [announcementId],
  );
  const submit = useCallback(
    (body: SurveyAnswersRequest) =>
      submitParentSurveyAnswers(announcementId, body),
    [announcementId],
  );
  const handleSubmitted = useCallback(() => {
    onUpdated(announcementId, {
      read: true,
      survey: { ...survey, participated: true },
    });
    window.dispatchEvent(new Event("parent-news-unread-refresh"));
  }, [announcementId, onUpdated, survey]);

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <SurveyAnswerPanel
        load={load}
        submit={submit}
        respondent="parent"
        reloadKey={`${announcementId}:${item.published_at ?? ""}`}
        showIntro={false}
        onBusyChange={onBusyChange}
        onSubmitted={handleSubmitted}
      />
    </section>
  );
}
