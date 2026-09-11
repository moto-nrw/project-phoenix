import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper.server";

interface FamilyProtectionBody {
  enabled?: boolean;
  reason?: string;
}

interface FamilyProtectionResponse {
  data: { student_id: string; enabled: boolean; reason: string };
}

export const GET = createGetHandler(async (_request, token, params) => {
  const id = params.id as string;
  if (!id) throw new Error("Kind ist erforderlich.");
  const response = await apiGet<FamilyProtectionResponse>(
    `/api/students/${encodeURIComponent(id)}/family-protection`,
    token,
  );
  return response.data;
});

export const PUT = createPutHandler<unknown, FamilyProtectionBody>(
  async (_request, body, token, params) => {
    const id = params.id as string;
    if (!id || typeof body.enabled !== "boolean" || !body.reason?.trim()) {
      throw new Error("Kind, Status und Begründung sind erforderlich.");
    }
    const response = await apiPut<{ data: unknown }>(
      `/api/students/${encodeURIComponent(id)}/family-protection`,
      token,
      { enabled: body.enabled, reason: body.reason.trim() },
    );
    return response.data;
  },
);
