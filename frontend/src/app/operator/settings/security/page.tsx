"use client";

import { Suspense } from "react";
// eslint-disable-next-line no-restricted-imports -- operator routes are not tenant-scoped
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { Loading } from "~/components/ui/loading";
import { MFASecuritySettings } from "~/components/auth/mfa-security-settings";
import { operatorPath } from "~/lib/operator-url";

function SecurityContent() {
  const { data: session, status } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (status === "unauthenticated") {
      router.push(operatorPath("/operator/login"));
    }
  }, [status, router]);

  if (status === "loading" || !session?.user?.token) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 px-4 py-6 md:px-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Sicherheit</h1>
        <p className="mt-1 text-sm text-gray-600">
          Verwalten Sie die Zwei-Faktor-Authentifizierung für Ihr
          Operator-Konto.
        </p>
      </header>

      <MFASecuritySettings
        scope="operator"
        bearerToken={session.user.token}
        userEmail={session.user.email ?? ""}
      />
    </div>
  );
}

export default function OperatorSecuritySettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SecurityContent />
    </Suspense>
  );
}
