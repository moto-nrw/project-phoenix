import "server-only";
import { createLoginRoute } from "./login-route.server";
import {
  setOperatorSession,
  withOperatorSessionResponse,
} from "~/server/auth/operator";
import { PORTALS, type BackendAuthStep } from "~/server/auth/portals";
import {
  setSchoolSession,
  withSchoolSessionResponse,
} from "~/server/auth/school";
import {
  setTraegerSession,
  withTraegerSessionResponse,
} from "~/server/auth/traeger";
import {
  setCatererSession,
  withCatererSessionResponse,
} from "~/server/auth/caterer";

const bindings = {
  operator: {
    withResponse: withOperatorSessionResponse,
    setSession: setOperatorSession,
  },
  school: {
    withResponse: withSchoolSessionResponse,
    setSession: setSchoolSession,
  },
  traeger: {
    withResponse: withTraegerSessionResponse,
    setSession: setTraegerSession,
    organizations: true,
  },
  caterer: {
    withResponse: withCatererSessionResponse,
    setSession: setCatererSession,
  },
};

export function createPortalLoginRoute(
  key: keyof typeof bindings,
  step: Exclude<BackendAuthStep, "refresh">,
) {
  const { backend } = PORTALS[key];
  return createLoginRoute(`${backend.authPrefix}/${step}`, {
    ...bindings[key],
    logout: backend.logout,
    envelope: (backend.enveloped as readonly BackendAuthStep[]).includes(step),
    challenges: step === "login",
  });
}
