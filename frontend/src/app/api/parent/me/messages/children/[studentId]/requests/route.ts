import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface CreateRequestBody {
  request_type: string;
  payload: Record<string, unknown>;
}

export const POST = createParentPostHandler<unknown, CreateRequestBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<unknown, CreateRequestBody>(
      `/parent/me/messages/children/${encodeURIComponent(studentId)}/requests`,
      token,
      body,
    );
  },
);
