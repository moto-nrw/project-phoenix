import { apiDelete, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";

interface PartialAbsenceBody {
  date: string;
  from_time: string;
  reason?: string;
}

export const PUT = createPutHandler<unknown, PartialAbsenceBody>(
  async (_request, body, token, params) => {
    const id = params.id as string;
    const partialAbsenceId = params.partialAbsenceId as string;
    if (!id || !partialAbsenceId) {
      throw new Error("Student ID and partial absence ID are required");
    }
    const response = await apiPut<{ data: unknown }, PartialAbsenceBody>(
      `/api/students/${id}/partial-absences/${partialAbsenceId}`,
      token,
      body,
    );
    return response.data;
  },
);

export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const id = params.id as string;
  const partialAbsenceId = params.partialAbsenceId as string;
  if (!id || !partialAbsenceId) {
    throw new Error("Student ID and partial absence ID are required");
  }
  await apiDelete(
    `/api/students/${id}/partial-absences/${partialAbsenceId}`,
    token,
  );
  return { deleted: true };
});
