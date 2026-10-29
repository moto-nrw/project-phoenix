import "server-only";
import { createLoginRoute } from "./login-route.server";
import {
  setOperatorSession,
  withOperatorSessionResponse,
} from "~/server/auth/operator";
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
    rollbackPath: "/operator/auth/logout",
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
  portal: keyof typeof bindings,
  step: "login" | "mfa/verify" | "mfa/enroll/confirm" | "passkeys/login/verify",
) {
  const path =
    portal === "operator"
      ? `/operator/auth/${step}`
      : `/${portal}/auth/${step}`;
  return createLoginRoute(path, {
    ...bindings[portal],
    // Operator passkey verification returns a plain LoginResponse.
    envelope:
      portal === "caterer" ||
      (portal === "operator" && step !== "passkeys/login/verify"),
    challenges: step === "login",
  });
}
