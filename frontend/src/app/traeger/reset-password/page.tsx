"use client";

import { Suspense } from "react";
import {
  DEFAULT_RESET_PASSWORD_PAGE_COPY,
  ResetPasswordPageContent,
} from "~/components/auth/reset-password-page-content";
import { AuthShellSkeleton } from "~/components/auth/auth-shell";
import { confirmTraegerPasswordReset } from "~/lib/auth-api";
import { TRAEGER_FORGOT_PASSWORD_PARAM } from "~/lib/traeger-login-params";
import { traegerPath } from "~/lib/traeger-url";

// Without a token the page shows no form, only the way to a new link: the
// login page opens „Passwort vergessen?“ right away.
const TRAEGER_RESET_COPY = {
  ...DEFAULT_RESET_PASSWORD_PAGE_COPY,
  missingToken:
    "Der Link ist unvollständig. Bitte fordern Sie einen neuen Link an.",
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
        noTokenAction={{
          label: "Neuen Link anfordern",
          href: `${loginPath}?${TRAEGER_FORGOT_PASSWORD_PARAM}=1`,
          subtitle: "Dafür brauchen Sie einen neuen Link.",
        }}
      />
    </Suspense>
  );
}
