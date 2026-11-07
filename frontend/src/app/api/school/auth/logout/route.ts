import { createLogoutHandler } from "~/lib/logout-route.server";
import {
  schoolAuth,
  uncachedSchoolAuth,
  withSchoolAuth,
} from "~/server/auth/school";
import { PORTALS } from "~/server/auth/portals";

export const POST = withSchoolAuth(
  createLogoutHandler(PORTALS.school, schoolAuth, uncachedSchoolAuth),
);
