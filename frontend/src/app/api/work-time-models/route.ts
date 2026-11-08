import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";

type WorkTimeModel = Record<string, unknown>;
type CreateBody = Record<string, unknown>;

// GET /api/work-time-models — list templates for the active tenant.
export const GET = proxyGet<WorkTimeModel[]>("/api/work-time-models");

// POST /api/work-time-models — create a new tenant template.
export const POST = proxyPost<WorkTimeModel, CreateBody>(
  "/api/work-time-models",
);
