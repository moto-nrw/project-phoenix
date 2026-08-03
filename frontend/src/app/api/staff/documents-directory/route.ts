import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface DirectoryEntry {
  id: number;
  name: string;
  firstName: string;
  lastName: string;
}

export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: DirectoryEntry[] }>(
    "/api/staff/documents-directory",
    token,
  );
  return (response.data ?? []).map((entry) => ({
    ...entry,
    id: entry.id.toString(),
  }));
});
