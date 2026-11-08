import "server-only";

export interface JwtPayload {
  id: string | number;
  exp?: number;
  token?: string;
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  role_is_system?: boolean[];
  permissions?: string[];
  is_admin?: boolean;
  tenant_id?: number;
  org_id?: number;
  scope?: string;
  read_only?: boolean;
  acting_admin_id?: number;
  /** An OGS session the Träger-Büro opened through the Absprung (#4654). */
  via_traeger?: boolean;
}

/**
 * The claims of a JWT, read without verifying it: for logging, metrics and
 * expiry checks only. Null when the token is not three parts or its payload
 * is no JSON object.
 */
export function decodeJwtPayload(token: string): JwtPayload | null {
  const parts = token.split(".");
  if (parts.length !== 3 || !parts[1]) return null;
  try {
    const payload: unknown = JSON.parse(
      Buffer.from(parts[1], "base64url").toString(),
    );
    return typeof payload === "object" && payload !== null
      ? (payload as JwtPayload)
      : null;
  } catch {
    return null;
  }
}
