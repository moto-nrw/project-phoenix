import { EmailConfirmContent } from "./email-confirm-content";

/**
 * Page for the email change confirmation flow.
 * The verification token arrives as a query parameter (?token=...) and is
 * stripped from the URL on mount via history.replaceState so it does not
 * persist in the address bar, browser history, or any subsequent Referer
 * header. Consumption requires an explicit click on the confirm button,
 * so link scanners cannot pre-consume the token.
 */
export default function OperatorEmailConfirmPage() {
  return <EmailConfirmContent />;
}
