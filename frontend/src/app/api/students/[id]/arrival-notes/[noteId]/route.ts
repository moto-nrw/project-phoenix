import { createDeleteHandler, createPutHandler } from "@/lib/route-wrapper";
import { apiDelete, apiPut } from "@/lib/api-helpers";

// PUT /api/students/[id]/arrival-notes/[noteId] - Update an arrival note
export const PUT = createPutHandler(async (_request, body, token, params) => {
  const { id, noteId } = params;

  const response = await apiPut(
    `/api/students/${String(id)}/arrival-notes/${String(noteId)}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});

// DELETE /api/students/[id]/arrival-notes/[noteId] - Delete an arrival note
export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const { id, noteId } = params;

  const response = await apiDelete(
    `/api/students/${String(id)}/arrival-notes/${String(noteId)}`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});
