"use client";

import { Suspense } from "react";
import {
  DEFAULT_RESET_PASSWORD_PAGE_COPY,
  ResetPasswordPageContent,
} from "~/components/auth/reset-password-page-content";
import { AuthShellSkeleton } from "~/components/auth/auth-shell";
import { confirmTraegerPasswordReset } from "~/lib/auth-api";
import { traegerPath } from "~/lib/traeger-url";

// Without a token the page names the way to a new link: the login page's
// „Passwort vergessen?“, reached through the back link above the form.
const TRAEGER_RESET_COPY = {
  ...DEFAULT_RESET_PASSWORD_PAGE_COPY,
  missingToken:
    "Der Link ist unvollständig. Einen neuen Link bekommen Sie bei der Anmeldung über „Passwort vergessen?“.",
};

export default function TraegerResetPasswordPage() {
  const loginPath = traegerPath("/traeger/login");

  return (
    <Suspense fallback={<AuthShellSkeleton />}>
      <ResetPasswordPageContent
        confirmReset={confirmTraegerPasswordReset}
        successRedirectPath={loginPath}
        backHref={loginPath}
        backLabel="Zurück zur Anmeldung"
        copy={TRAEGER_RESET_COPY}
      />
    </Suspense>
  );
}
