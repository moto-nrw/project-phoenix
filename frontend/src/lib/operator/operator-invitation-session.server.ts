export const OPERATOR_INVITATION_COOKIE = "operator.invitation-token";

export const operatorInvitationCookieOptions = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "strict",
  path: "/api/operator/auth/invitations",
  maxAge: 60 * 60,
  priority: "high",
} as const;
