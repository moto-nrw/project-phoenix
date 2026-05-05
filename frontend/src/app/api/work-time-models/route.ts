import type { NextRequest } from "next/server";

import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

type WorkTimeModel = Record<string, unknown>;
type CreateBody = Record<string, unknown>;

// GET /api/work-time-models — list templates for the active tenant.
export const GET = createGetHandler<WorkTimeModel[]>(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: WorkTimeModel[] }>(
      "/api/work-time-models",
      token,
    );
    return response.data;
  },
);

// POST /api/work-time-models — create a new tenant template.
export const POST = createPostHandler<WorkTimeModel, CreateBody>(
  async (_request: NextRequest, body: CreateBody, token: string) => {
    const response = await apiPost<{ data: WorkTimeModel }>(
      "/api/work-time-models",
      token,
      body,
    );
    return response.data;
  },
);
