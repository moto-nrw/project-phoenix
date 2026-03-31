import { EmailConfirmContent } from "./email-confirm-content";

/**
 * Server component page that resolves the token from searchParams
 * before passing it to the client component. This avoids useSearchParams
 * hydration mismatches entirely.
 */
export default async function OperatorEmailConfirmPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;
  return <EmailConfirmContent token={token ?? null} />;
}
