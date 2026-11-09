import { schoolPortal } from "~/server/auth/school";
import { makePortalHandlers } from "../portal-handlers.server";

/** moto schule's route handlers (#2207), on the school session only. */
export const school = makePortalHandlers(schoolPortal);

export const {
  proxyGet,
  proxyPost,
  proxyPut,
  proxyPatch,
  proxyDelete,
  del: createSchoolDeleteHandler,
} = school;
