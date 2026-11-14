import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (request: NextRequest, token, params) => {
    const appointmentId = params.appointmentId as string;
    // occurrence_date fragt nach den Antworten eines Termins einer Reihe (#6987).
    const occurrenceDate = request.nextUrl.searchParams.get("occurrence_date");
    const query = occurrenceDate
      ? `?${new URLSearchParams({ occurrence_date: occurrenceDate }).toString()}`
      : "";
    const response = await apiGet<{ data: unknown }>(
      `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/overview${query}`,
      token,
    );
    return response.data;
  },
);
