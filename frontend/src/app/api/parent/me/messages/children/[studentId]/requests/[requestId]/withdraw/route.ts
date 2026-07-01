import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

export const POST = createParentPostHandler<unknown, Record<string, never>>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    const requestId = String(params.requestId);
    return parentApiPost<unknown, Record<string, never>>(
      `/parent/me/messages/children/${encodeURIComponent(studentId)}/requests/${encodeURIComponent(requestId)}/withdraw`,
      token,
      body,
    );
  },
);
