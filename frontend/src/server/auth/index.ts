import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";
import { sessionTenantMismatchResponse } from "./tenant-host-check";

const tenant = createPortalAuth(PORTALS.tenant, (request) =>
  // A session of another school than the request's subdomain is refused
  // before the handler runs (#5130).
  sessionTenantMismatchResponse(request, request.auth),
);

/** The tenant portal, whose cookie spans every school subdomain. */
export const tenantPortal = {
  ...tenant,
  async setStaffPreviewSession(preview: {
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
  },
};
