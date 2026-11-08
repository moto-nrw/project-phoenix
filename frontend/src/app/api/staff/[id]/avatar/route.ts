import { tenant } from "~/lib/route-wrapper.server";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

// `staffAvatarUrl` adds `?v=<stored filename>`. Each upload gets a new
// filename, so a versioned address never changes its bytes and the browser
// may keep it without revalidation. Unversioned addresses stay short-lived.
const VERSIONED_CACHE_CONTROL = "private, max-age=86400, immutable";
const UNVERSIONED_CACHE_CONTROL = "private, max-age=300";

export const GET = tenant.route((request, _token, params) =>
  tenant.downloadFile(
    request,
    `/api/staff/${requireNumericParam(params)}/avatar`,
    {
      fallbackType: "image/jpeg",
      cacheControl: request.nextUrl.searchParams.get("v")
        ? VERSIONED_CACHE_CONTROL
        : UNVERSIONED_CACHE_CONTROL,
    },
  ),
);
