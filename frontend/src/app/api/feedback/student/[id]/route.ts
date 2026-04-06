import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

interface BackendFeedbackEntry {
  id: number;
  value: "positive" | "neutral" | "negative";
  day: string;
  time: string;
  student_id: number;
  is_mensa_feedback: boolean;
  created_at: string;
  updated_at: string;
  student?: {
    id: number;
    first_name: string;
    last_name: string;
  };
}

interface BackendFeedbackResponse {
  status: string;
  data: BackendFeedbackEntry[];
  message: string;
}

export const GET = createGetHandler(async (_request, token: string, params) => {
  const studentId = params.id as string;
  const response = await apiGet<BackendFeedbackResponse>(
    `/api/feedback/student/${studentId}`,
    token,
  );
  return response.data;
});
