// The password step of a portal whose BFF installs the session itself and
// whose account may have a voluntary second factor: the caterer portal
// (#4497) and the parents portal (#7051). The browser learns only whether a
// second step follows. A refusal comes back as an ApiError with its code.

import { apiErrorFromResponse, transportFetch } from "./api-error";
import { portalAuthAnswer, portalAuthPath } from "./portal-paths";

export type PortalPasswordLoginResult =
  | { status: "authenticated" }
  | {
      status: "mfa_required";
      challengeToken: string;
      maskedEmail: string;
      trustedDeviceEnabled: boolean;
      trustedDeviceDays?: number;
    };

interface BackendPasswordLogin {
  status?: "authenticated" | "mfa_required";
  challenge_token?: string;
  masked_email?: string;
  trusted_device_enabled?: boolean;
  trusted_device_days?: number;
}

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
  const data = portalAuthAnswer<BackendPasswordLogin>(
    portal,
    "login",
    await response.json(),
  );
  if (data.status === "mfa_required") {
    return {
      status: "mfa_required",
      challengeToken: data.challenge_token ?? "",
      maskedEmail: data.masked_email ?? "",
      trustedDeviceEnabled: data.trusted_device_enabled ?? false,
      trustedDeviceDays: data.trusted_device_days,
    };
  }
  return { status: "authenticated" };
}
