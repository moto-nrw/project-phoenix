import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Anfragen-Modul, Reiter Mitarbeitende (#2433): offene Abwesenheitsanträge
// oder deren Historie, mit Namenssuche und Art-Filter. Backend gates on
// vacation:approve.
export const GET = createGetHandler(async (request, token) => {
  const search = new URL(request.url).searchParams;
  const params = new URLSearchParams();
  params.set("view", search.get("view") === "history" ? "history" : "open");
  const term = search.get("search");
  if (term) params.set("search", term);
  const types = search.get("types");
  if (types) params.set("types", types);
  const response = await apiGet<{ data: unknown }>(
    `/api/staff/absences/requests?${params.toString()}`,
    token,
  );
  return response.data;
});
