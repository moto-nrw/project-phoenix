import { revalidatePath, revalidateTag } from "next/cache";
import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers.server";
import { createPutHandler } from "~/lib/route-wrapper.server";
import { tenantSlugFromHost } from "~/lib/tenant-host";

// Die Antworten des ersten Schritts (#2832). Die Anwesenheitsart reist über
// /auth/tenant/resolve in die Oberfläche; ohne Revalidierung zeigten neue
// Tabs bis zu 300 s lang die alte Art.
export const PUT = createPutHandler(
  async (request: NextRequest, body: unknown, token: string) => {
    // Wie die übrigen Routen des Assistenten (proxyPut): die Hülle `data` der
    // Backend-Antwort abnehmen. Sonst käme der Stand doppelt verpackt an und
    // der Assistent läse ihn als „keine Schritte“.
    const result = await apiPut<{ data?: unknown }>(
      "/api/school-setup/basics",
      token,
      body,
    );
    const slug = tenantSlugFromHost(request.headers.get("host"));
    if (slug) {
      revalidateTag(`tenant-${slug}`, { expire: 0 });
      revalidatePath(`/${slug}`, "layout");
    }
    return result.data;
  },
);
