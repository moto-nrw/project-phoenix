// app/api/students/[id]/purge/route.ts
//
// Hard-deletes a child a grade transition graduated. Deliberately its own route
// rather than a flag on the students DELETE: the backend gate that hides
// alumni from every ordinary student route is what this one bypasses, so it
// stays greppable and cannot be reached by a stray query parameter.
import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token, params): Promise<null> => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid id parameter");
    }
    await apiDelete(`/api/students/${id}/purge`, token);
    return null;
  },
);
