/**
 * The one-time link of a care contract mail (#4679): "<Träger>.<token>",
 * the Träger as a number, the token as base64url. A malformed link never
 * reaches the backend.
 */
export function isCareContractSigningLink(link: string): boolean {
  return /^[1-9]\d{0,18}\.[A-Za-z0-9_-]{16,128}$/.test(link);
}
