import { randomUUID } from "node:crypto";
import type { NextRequest } from "next/server";

import { OPERATOR_INVITATION_FLOW_HEADER } from "./operator-invitation-flow";

const OPERATOR_INVITATION_COOKIE_PREFIX = "operator.invitation-token";
const OPERATOR_INVITATION_FLOW_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function createOperatorInvitationFlowID(): string {
  return randomUUID();
}

export function operatorInvitationCookieName(flowID: string): string {
  if (!OPERATOR_INVITATION_FLOW_ID_RE.test(flowID)) {
    throw new Error("invalid operator invitation flow ID");
  }
  return `${OPERATOR_INVITATION_COOKIE_PREFIX}.${flowID}`;
}

export function operatorInvitationFlowID(request: NextRequest): string {
  const flowID = request.headers.get(OPERATOR_INVITATION_FLOW_HEADER) ?? "";
  operatorInvitationCookieName(flowID);
  return flowID;
}

export function operatorInvitationToken(request: NextRequest): string {
  const flowID = operatorInvitationFlowID(request);
  return request.cookies.get(operatorInvitationCookieName(flowID))?.value ?? "";
}

export const operatorInvitationCookieOptions = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "strict",
  path: "/api/operator/auth/invitations",
  // The backend remains authoritative for the actual invitation expiry. The
  // cookie only needs to avoid expiring before the configured 168-hour cap.
  maxAge: 60 * 60 * 24 * 7,
  priority: "high",
} as const;
