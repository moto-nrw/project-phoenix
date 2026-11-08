import { bffError } from "~/lib/api-response-error.server";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requireStringParam } from "~/lib/route-wrapper-utils.server";

/** A stored avatar file name; anything that could name a path is refused. */
function avatarFilename(params: Record<string, unknown>): string {
  const filename = requireStringParam(params, "filename");
  if (/\.\.|[/\\]/.test(filename)) throw bffError(400, "Invalid filename");
  return encodeURIComponent(filename);
}

/** The signed-in account's avatar; the browser keeps it for a day. */
export const GET = proxyFile(
  (params) => `/api/me/profile/avatar/${avatarFilename(params)}`,
  { fallbackType: "image/jpeg", cacheControl: "private, max-age=86400" },
);
