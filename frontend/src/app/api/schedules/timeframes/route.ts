// app/api/schedules/timeframes/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TimeframesRoute" });

// Define the backend timeframe interface based on schedule.timeframes table
interface BackendTimeframe {
  id: number;
  start_time: string;
  end_time?: string;
  is_active: boolean;
  description?: string;
  created_at: string;
  updated_at: string;
}

// Frontend timeframe interface
interface Timeframe {
  id: string;
  start_time: string;
  end_time?: string;
  is_active: boolean;
  description?: string;
  display_name?: string; // Computed field for display
}

// Helper function to format time for display
function formatTimeRange(startTime: string, endTime?: string): string {
  try {
    const start = new Date(startTime);
    const startFormatted = start.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });

    if (endTime) {
      const end = new Date(endTime);
      const endFormatted = end.toLocaleTimeString("de-DE", {
        hour: "2-digit",
        minute: "2-digit",
      });
      return `${startFormatted} - ${endFormatted}`;
    }

    return startFormatted;
  } catch {
    return endTime ? `${startTime} - ${endTime}` : startTime;
  }
}

// Mapping function for timeframes
function mapTimeframeResponse(timeframe: BackendTimeframe): Timeframe {
  const displayName =
    timeframe.description ??
    formatTimeRange(timeframe.start_time, timeframe.end_time);

  return {
    id: String(timeframe.id),
    start_time: timeframe.start_time,
    end_time: timeframe.end_time,
    is_active: timeframe.is_active,
    description: timeframe.description,
    display_name: displayName,
  };
}

/**
 * Handler for GET /api/schedules/timeframes
 * Returns a list of available timeframes from schedule.timeframes table
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    try {
      // Call the backend schedules API for timeframes
      const response = await apiGet<{
        status: string;
        data: BackendTimeframe[];
      }>("/api/schedules/timeframes", token);

      // Handle response structure
      if (response?.status === "success" && Array.isArray(response.data)) {
        // Filter only active timeframes and map them
        return response.data
          .filter((tf) => tf.is_active)
          .map(mapTimeframeResponse);
      }

      // If no data or unexpected structure, handle safely
      if (response && Array.isArray(response.data)) {
        return response.data
          .filter((tf) => tf.is_active)
          .map(mapTimeframeResponse);
      }

      logger.error("unexpected timeframe response format");
      return [];
    } catch (error) {
      logger.error("timeframes fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return [];
    }
  },
);
