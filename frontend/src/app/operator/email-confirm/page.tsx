import { EmailConfirmContent } from "./email-confirm-content";

/**
 * Page for the email change confirmation flow.
 * The verification token arrives via URL fragment (#token=...) to prevent
 * leaking in Referer headers and server logs — fragments are never sent
 * in HTTP requests. The client component extracts it on mount.
 */
export default function OperatorEmailConfirmPage() {
  return <EmailConfirmContent />;
}
