import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface SurveyQuestionView {
  id: string;
  position: number;
  kind: string;
  prompt: string;
  required: boolean;
  options: { id: string; label: string }[];
}

interface SurveyFormView {
  survey: {
    id: string;
    title: string;
    intro: string;
    anonymous: boolean;
    closes_at: string;
    questions: SurveyQuestionView[];
  };
  school_name: string;
  participated: boolean;
}

interface SurveyAnswersBody {
  answers?: {
    question_id: string;
    option_ids?: string[];
    scale_value?: number;
    text?: string;
  }[];
}

function surveyPath(params: Record<string, unknown>): string {
  return `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/survey`;
}

/**
 * Proxy /api/parent/me/news/{announcementId}/survey → backend (#4781). GET
 * returns the questionnaire of an Umfrage in the guardian's feed, POST submits
 * the answers once per parent account. The body passes through untouched and
 * is never logged here.
 */
export const GET = proxyGet<SurveyFormView>(surveyPath);
export const POST = proxyPost<{ submitted: boolean }, SurveyAnswersBody>(
  surveyPath,
);
