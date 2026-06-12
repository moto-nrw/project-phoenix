import {
  createParentGetHandler,
  createParentPutHandler,
  parentApiGet,
  parentApiPut,
} from "~/lib/parent/route-wrapper.server";

// portal_locale is null when the guardian has never picked a parents-portal
// language; the client then keeps the anonymous cookie/Accept-Language locale.
interface BackendParentProfile {
  portal_locale: string | null;
}

interface UpdateParentProfileBody {
  portal_locale: string;
}

export const GET = createParentGetHandler<BackendParentProfile>(
  async (_request, token) =>
    parentApiGet<BackendParentProfile>("/parent/me/profile", token),
);

export const PUT = createParentPutHandler<
  BackendParentProfile,
  UpdateParentProfileBody
>(async (_request, body, token) =>
  parentApiPut<BackendParentProfile, UpdateParentProfileBody>(
    "/parent/me/profile",
    token,
    body,
  ),
);
