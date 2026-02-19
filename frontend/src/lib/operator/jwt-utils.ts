/**
 * Extract the `exp` claim from a JWT token without cryptographic verification.
 * Tokens are trusted here because they were just issued by our backend.
 *
 * @returns Expiry timestamp in milliseconds, or null if extraction fails.
 */
export function extractJwtExpiry(token: string): number | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    const payload = parts[1];
    if (!payload) return null;

    const decoded = JSON.parse(
      Buffer.from(payload, "base64url").toString("utf-8"),
    ) as { exp?: number };

    if (typeof decoded.exp !== "number") return null;

    return decoded.exp * 1000;
  } catch {
    return null;
  }
}
