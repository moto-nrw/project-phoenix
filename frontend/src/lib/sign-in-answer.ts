// What a portal's password step answers, in one shape for every login page.
// Each portal keeps its own transport (the Träger names its Träger, the
// caterer and the parents portal read the refusal from the response); the
// reading of the answer is here.

import { apiErrorFromBody } from "./api-error";
import type { PortalKey } from "./portal-paths";

/** The code step a login page keeps between the password and the code. */
export interface MFAStep {
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly trustedDeviceEnabled: boolean;
  /** Missing: the code form shows the registry default. */
  readonly trustedDeviceDays?: number;
  /** The Träger the Träger-Portal sign-in resolved (#4651); "" elsewhere. */
  readonly organizationId: string;
}

/** A Träger the person can open, offered when they belong to several. */
export interface TraegerChoice {
  readonly id: string;
  readonly name: string;
}

export type SignInAnswer =
  | { readonly status: "authenticated" }
  | ({ readonly status: "mfa_required" } & MFAStep)
  | {
      /**
       * The account sets up its second factor first. The token authorizes
       * only the portal's enrollment routes; the confirm mints the session.
       */
      readonly status: "mfa_enrollment_required";
      readonly enrollmentToken: string;
      readonly maskedEmail: string;
      readonly organizationId: string;
    }
  | {
      readonly status: "organization_selection_required";
      readonly organizations: readonly TraegerChoice[];
    };

interface WireSignInAnswer {
  status?: string;
  access_token?: string;
  challenge_token?: string;
  masked_email?: string;
  organization_id?: number;
  organizations?: { id: number; name: string }[];
  trusted_device_enabled?: boolean;
  trusted_device_days?: number;
}

interface SignInWire {
  /**
   * What an answer without `trusted_device_enabled` means. For tenant,
   * school and operator only a backend from before the field omits it: the
   * device may be remembered. The caterer omits a false flag (`omitempty`),
   * and the parents portal and the Träger-Portal offer nothing to remember
   * when the field is missing or unreadable (#6955).
   */
  readonly rememberDeviceWhenOmitted: boolean;
  /**
   * The Träger-Portal refuses an answer it cannot read (502): an unknown
   * status, or a step without its token. The other portals take a step as
   * sent, with an empty token, and everything else as signed in; their BFF
   * has refused such an answer before.
   */
  readonly refusesUnreadable: boolean;
}

const REMEMBER_WHEN_OMITTED: SignInWire = {
  rememberDeviceWhenOmitted: true,
  refusesUnreadable: false,
};
const FORGET_WHEN_OMITTED: SignInWire = {
  rememberDeviceWhenOmitted: false,
  refusesUnreadable: false,
};

const SIGN_IN_WIRE: Record<PortalKey, SignInWire> = {
  tenant: REMEMBER_WHEN_OMITTED,
  school: REMEMBER_WHEN_OMITTED,
  operator: REMEMBER_WHEN_OMITTED,
  caterer: FORGET_WHEN_OMITTED,
  parent: FORGET_WHEN_OMITTED,
  traeger: { rememberDeviceWhenOmitted: false, refusesUnreadable: true },
};

/** Reads the answer of a portal's password step, out of its envelope. */
export function signInAnswer(portal: PortalKey, body: unknown): SignInAnswer {
  const wire = SIGN_IN_WIRE[portal];
  const answer = (body ?? {}) as WireSignInAnswer;
  const organizationId = String(answer.organization_id ?? "");
  const readable = (field?: string) => !wire.refusesUnreadable || !!field;

  if (answer.status === "organization_selection_required") {
    return {
      status: "organization_selection_required",
      organizations: (answer.organizations ?? []).map((organization) => ({
        id: String(organization.id),
        name: organization.name,
      })),
    };
  }
  if (answer.status === "mfa_required" && readable(answer.challenge_token)) {
    return {
      status: "mfa_required",
      challengeToken: answer.challenge_token ?? "",
      maskedEmail: answer.masked_email ?? "",
      organizationId,
      trustedDeviceEnabled:
        answer.trusted_device_enabled ?? wire.rememberDeviceWhenOmitted,
      trustedDeviceDays: answer.trusted_device_days,
    };
  }
  if (
    answer.status === "mfa_enrollment_required" &&
    readable(answer.access_token)
  ) {
    return {
      status: "mfa_enrollment_required",
      enrollmentToken: answer.access_token ?? "",
      maskedEmail: answer.masked_email ?? "",
      organizationId,
    };
  }
  if (wire.refusesUnreadable && answer.status !== "authenticated") {
    throw apiErrorFromBody("Invalid login response", 502, body);
  }
  return { status: "authenticated" };
}
