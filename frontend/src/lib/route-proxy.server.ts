import { tenant } from "./route-wrapper.server";

/** The tenant's pure proxy routes: `export const GET = proxyGet(path)`. */
export const { proxyGet, proxyPost, proxyPut, proxyPatch, proxyDelete } =
  tenant;
