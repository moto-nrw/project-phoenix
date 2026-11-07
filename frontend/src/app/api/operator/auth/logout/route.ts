import { createLogoutHandler } from "~/lib/logout-route.server";
import {
  operatorAuth,
  uncachedOperatorAuth,
  withOperatorAuth,
} from "~/server/auth/operator";
import { PORTALS } from "~/server/auth/portals";

export const POST = withOperatorAuth(
  createLogoutHandler(PORTALS.operator, operatorAuth, uncachedOperatorAuth),
);
