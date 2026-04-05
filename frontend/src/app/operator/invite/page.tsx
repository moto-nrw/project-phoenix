import { InviteContent } from "./invite-content";

/**
 * Public page for accepting operator invitations.
 * The token arrives as a query parameter (?token=...) and is stripped from
 * the URL on mount via history.replaceState so it does not persist in the
 * address bar, browser history, or any subsequent Referer header.
 */
export default function OperatorInvitePage() {
  return <InviteContent />;
}
