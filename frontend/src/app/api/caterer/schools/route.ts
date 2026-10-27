import { proxyGet } from "~/lib/caterer/route-wrapper.server";
import type { BackendCatererSchools } from "~/lib/caterer-api";

/** The caterer and the schools it supplies now. */
export const GET = proxyGet<BackendCatererSchools>("/caterer/schools");
