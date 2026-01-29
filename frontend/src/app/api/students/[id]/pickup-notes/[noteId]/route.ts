import { createPutHandler, createDeleteHandler } from "@/lib/route-wrapper";
import { apiPut, apiDelete } from "@/lib/api-helpers";

// PUT /api/students/[id]/pickup-notes/[noteId] - Update a pickup note
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id, noteId } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/pickup-notes/${String(noteId)}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// DELETE /api/students/[id]/pickup-notes/[noteId] - Delete a pickup note
export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const { id, noteId } = params;

  await apiDelete(
    `/api/students/${String(id)}/pickup-notes/${String(noteId)}`,
    token,
  );
  return null;
});
