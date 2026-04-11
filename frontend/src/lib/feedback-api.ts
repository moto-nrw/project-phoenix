import { sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "FeedbackAPI" });

export interface BackendFeedbackEntry {
  id: number;
  value: "positive" | "neutral" | "negative";
  day: string;
  time: string;
  student_id: number;
  is_mensa_feedback: boolean;
  created_at: string;
  updated_at: string;
}

export interface FeedbackEntry {
  id: string;
  timestamp: string;
  feedback_type: "positive" | "neutral" | "negative";
  is_mensa_feedback: boolean;
}

function mapFeedbackResponse(entry: BackendFeedbackEntry): FeedbackEntry {
  // Combine day + time into a timestamp
  // day is "2026-04-06", time is "15:30:23"
  const timestamp = `${entry.day}T${entry.time}`;

  return {
    id: entry.id.toString(),
    timestamp,
    feedback_type: entry.value,
    is_mensa_feedback: entry.is_mensa_feedback,
  };
}

export async function fetchStudentFeedback(
  studentId: string,
): Promise<FeedbackEntry[]> {
  const url = `/api/feedback/student/${studentId}`;

  try {
    const response = await sessionFetch(url, {
      method: "GET",
      credentials: "include",
    });

    if (!response.ok) {
      if (response.status === 404) return [];
      if (response.status === 403) {
        throw new Error("feature_disabled");
      }
      throw new Error(
        `Failed to fetch feedback: ${response.status} ${response.statusText}`,
      );
    }

    const responseData = (await response.json()) as {
      data: BackendFeedbackEntry[];
    };
    const entries = responseData.data ?? [];
    return entries.map(mapFeedbackResponse);
  } catch (error) {
    logger.error("fetch_student_feedback_failed", {
      error: error instanceof Error ? error.message : String(error),
      student_id: studentId,
    });
    throw error;
  }
}
