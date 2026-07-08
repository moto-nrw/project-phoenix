// app/api/displays/[id]/regenerate/route.ts — mint a new display token
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface RegenerateResponse {
  token: string;
}

export const POST = createPostHandler<RegenerateResponse, undefined>(
  async (_request, _body, token, params) => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("invalid display id");
    }
    return apiPost<RegenerateResponse>(`/api/display/${id}/regenerate`, token);
  },
);
