// app/api/rooms/[id]/students-present/route.ts
//
// Proxy for GET /api/rooms/{id}/students-present.
//
// Pass-through proxy to the backend — the response shape is already what the
// frontend consumes after the camelCase mapping in
// lib/students-in-room-helpers.ts. We do not normalise on this hop because
// the redaction is server-side: any field the caller is not authorised to
// see is already null on the wire, and we must not invent defaults that
// would silently un-redact a value.
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import { isStringParam } from "~/lib/route-wrapper-utils";
import type { BackendStudentsInRoomResponse } from "~/lib/students-in-room-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RoomStudentsPresentRoute" });

export const GET = createGetHandler<BackendStudentsInRoomResponse>(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    try {
      const response = await apiGet<{
        data?: BackendStudentsInRoomResponse;
      }>(`/api/rooms/${params.id}/students-present`, token);

      // Backend always wraps responses as { success, message, data } via
      // common.Respond — apiGet returns the parsed body. Unwrap once so the
      // route handler emits the inner payload, which createGetHandler will
      // re-wrap in its own ApiResponse envelope.
      if (
        response &&
        typeof response === "object" &&
        "data" in response &&
        response.data
      ) {
        return response.data;
      }

      // Defensive: if the backend ever changes shape, surface a real error
      // instead of silently returning garbage to SWR consumers.
      throw new Error("unexpected students-present response shape");
    } catch (error) {
      logger.error("students_present_fetch_failed", {
        room_id: params.id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);
