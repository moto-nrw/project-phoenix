import { proxyGet, proxyPut } from "~/lib/parent/route-wrapper.server";

// portal_locale is null when the guardian has never picked a parents-portal
// language; the client then keeps the anonymous cookie/Accept-Language locale.
interface BackendParentProfile {
  portal_locale: string | null;
}

interface UpdateParentProfileBody {
  portal_locale: string;
}

export const GET = proxyGet<BackendParentProfile>("/parent/me/profile");

export const PUT = proxyPut<BackendParentProfile, UpdateParentProfileBody>(
  "/parent/me/profile",
);
