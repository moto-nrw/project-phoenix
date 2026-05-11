"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Loading } from "~/components/ui/loading";
import { MFASecuritySettings } from "~/components/auth/mfa-security-settings";

function SecurityContent() {
  const { data: session, status } = useSession({ required: true });

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }
  if (!session?.user?.token) {
    redirect("/");
  }

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 px-4 py-6 md:px-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Sicherheit</h1>
        <p className="mt-1 text-sm text-gray-600">
          Verwalten Sie die Zwei-Faktor-Authentifizierung für Ihr Konto.
        </p>
      </header>

      <MFASecuritySettings
        scope="tenant"
        bearerToken={session.user.token}
        userEmail={session.user.email ?? ""}
      />
    </div>
  );
}

export default function SecuritySettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SecurityContent />
    </Suspense>
  );
}
