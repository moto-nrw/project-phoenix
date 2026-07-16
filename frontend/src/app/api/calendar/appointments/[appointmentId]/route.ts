import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
  isStringParam,
} from "~/lib/route-wrapper.server";

export const GET = createGetHandler(async (_request, token: string, params) => {
  const appointmentId = isStringParam(params.appointmentId)
    ? params.appointmentId
    : "";
  const response = await apiGet<{ data: unknown }>(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
    token,
  );
  return response.data;
});

export const PUT = createPutHandler(
  async (_request, body: unknown, token: string, params) => {
    const appointmentId = isStringParam(params.appointmentId)
      ? params.appointmentId
      : "";
    const response = await apiPut<{ data: unknown }>(
      `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
      token,
      body ?? {},
    );
    return response.data;
  },
);

export const DELETE = createDeleteHandler(
  async (_request, token: string, params) => {
    const appointmentId = isStringParam(params.appointmentId)
      ? params.appointmentId
      : "";
    await apiDelete(
      `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
      token,
    );
    return null;
  },
);
