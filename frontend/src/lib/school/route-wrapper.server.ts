import {
  schoolAuth,
  uncachedSchoolAuth,
  withSchoolAuth,
} from "~/server/auth/school";
import { unwrapEnvelope } from "../backend-client.server";
import { makePortalHandlers } from "../portal-handlers.server";

/** moto schule's route handlers (#2207), on the school session only. */
export const school = makePortalHandlers({
  key: "school",
  auth: () => schoolAuth(),
  uncachedAuth: () => uncachedSchoolAuth(),
  withAuth: (handler) => withSchoolAuth(handler),
});

export const {
  proxyGet,
  proxyPost,
  proxyPut,
  proxyPatch,
  proxyDelete,
  del: createSchoolDeleteHandler,
} = school;

export const schoolApiDelete = async <T>(endpoint: string, token: string) =>
  unwrapEnvelope<T>(await school.api("DELETE", endpoint, token));
