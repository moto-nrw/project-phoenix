// The password step of a portal whose BFF installs the session itself and
// whose account may have a voluntary second factor: the caterer portal
// (#4497) and the parents portal (#7051). The browser learns only whether a
// second step follows. A refusal comes back as an ApiError with its code.

import { apiErrorFromResponse, transportFetch } from "./api-error";
import { portalAuthAnswer, portalAuthPath } from "./portal-paths";
import { signInAnswer, type SignInAnswer } from "./sign-in-answer";

/** The caterer's and the parents' sign-in never asks for a setup step. */
export type PortalPasswordLoginResult = Extract<
  SignInAnswer,
  { status: "authenticated" | "mfa_required" }
>;

export async function portalPasswordLogin(
  portal: "caterer" | "parent",
  email: string,
  password: string,
): Promise<PortalPasswordLoginResult> {
  // The trusted-device cookie of this browser travels along (same origin),
  // so a remembered device skips the code.
  const response = await transportFetch(portalAuthPath(portal, "login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, `${portal} login failed`);
  }
  const answer = signInAnswer(
    portal,
    portalAuthAnswer<unknown>(portal, "login", await response.json()),
  );
  return answer.status === "mfa_required" ? answer : { status: "authenticated" };
}
