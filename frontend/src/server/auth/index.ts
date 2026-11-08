import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";
import { sessionTenantMismatchResponse } from "./tenant-host-check";

/** The tenant portal, whose cookie spans every school subdomain. */
const tenant = createPortalAuth(PORTALS.tenant);

export const { auth, handlers, setSession: setTenantSession } = tenant;

/**
 * Every tenant BFF route. A session of another school than the request's
 * subdomain is refused before the handler runs (#5130).
 */
export const withTenantAuth: typeof tenant.withAuthResponse = (handler) =>
  tenant.withAuthResponse(async (request, context) => {
    const mismatch = await sessionTenantMismatchResponse(request, request.auth);
    return mismatch ?? handler(request, context);
  });

/** The tenant portal, whose routes run in withTenantAuth. */
export const tenantPortal = { ...tenant, withAuth: withTenantAuth };

export async function setStaffPreviewSession(preview: {
  accessToken: string;
  expiresIn: number;
  targetAccountId: string;
  targetName: string;
}) {
  await tenant.updateSession(
    { previewStart: preview },
    (session) =>
      session?.user?.isPreview === true &&
      session.user.previewTargetAccountId === preview.targetAccountId,
  );
}
