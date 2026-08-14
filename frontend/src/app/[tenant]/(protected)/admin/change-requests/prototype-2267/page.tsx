import { notFound } from "next/navigation";

import { UnifiedParentRequestsPrototype } from "~/components/prototypes/unified-parent-requests-prototype";

/**
 * PROTOTYPE — throw away after #2267 has answered the interaction questions.
 * Three variants of the unified parent-request inbox, switchable via
 * `?variant=A|B|C`, inside the existing staff shell.
 */
export default function UnifiedParentRequestsPrototypePage() {
  if (process.env.NODE_ENV === "production") notFound();

  return <UnifiedParentRequestsPrototype />;
}
