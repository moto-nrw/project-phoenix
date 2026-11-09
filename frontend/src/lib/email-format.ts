// Loose on purpose: it catches typing slips, the backend decides. The top
// level domain has at least two characters; no real one is shorter.
const LOOSE_EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;

/** Whether `email`, trimmed, has the shape `x@y.zz`. */
export function looksLikeEmail(email: string): boolean {
  return LOOSE_EMAIL.test(email.trim());
}
