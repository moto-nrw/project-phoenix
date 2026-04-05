import { InviteContent } from "./invite-content";

/**
 * Public page for accepting operator invitations.
 * The token arrives via URL fragment (#token=...) to prevent leaking in
 * Referer headers and server logs. The client component extracts it on mount.
 */
export default function OperatorInvitePage() {
  return <InviteContent />;
}
