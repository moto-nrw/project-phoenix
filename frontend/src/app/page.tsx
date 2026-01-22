// app/page.tsx
// Root page - shows org selection on main domain, smart redirect on subdomains
"use client";

import { useEffect, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useSession } from "~/lib/auth-client";
import { OrgSelection } from "~/components/auth/org-selection";
import { SmartRedirect } from "~/components/auth/smart-redirect";
import { Loading } from "~/components/ui/loading";
import { Alert } from "~/components/ui";

function getBaseDomain(): string {
  return process.env.NEXT_PUBLIC_BASE_DOMAIN ?? "localhost:3000";
}

function isMainDomain(): boolean {
  if (typeof window === "undefined") {
    return true; // Server-side, assume main domain
  }

  const hostname = window.location.hostname;
  const baseDomain = getBaseDomain();

  // Handle localhost development
  if (baseDomain.startsWith("localhost")) {
    // Main domain if hostname is just "localhost"
    return hostname === "localhost" || !hostname.includes(".");
  }

  // Production: main domain if hostname matches base domain exactly
  const baseParts = baseDomain.replace(/:\d+$/, "").split(".");
  const hostParts = hostname.split(".");

  // Main domain if same number of parts (no subdomain)
  return hostParts.length === baseParts.length;
}

function RootPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, isPending: isSessionLoading } = useSession();
  const [isOnMainDomain, setIsOnMainDomain] = useState<boolean | null>(null);

  // Check domain on client side
  useEffect(() => {
    setIsOnMainDomain(isMainDomain());
  }, []);

  // Subdomain without session - redirect to /login (fallback, middleware handles this)
  useEffect(() => {
    if (isOnMainDomain === false && !session && !isSessionLoading) {
      router.push("/login");
    }
  }, [isOnMainDomain, session, isSessionLoading, router]);

  // Handle org status messages from redirects
  const orgStatus = searchParams.get("org_status");
  const statusMessages: Record<string, string> = {
    pending:
      "Ihre Einrichtung wird noch geprüft. Sie werden benachrichtigt, sobald sie freigeschaltet wurde.",
    rejected:
      "Ihre Einrichtung wurde leider abgelehnt. Bitte kontaktieren Sie den Support.",
    suspended:
      "Ihre Einrichtung wurde vorübergehend gesperrt. Bitte kontaktieren Sie den Support.",
    not_found: "Die angegebene Einrichtung wurde nicht gefunden.",
  };

  // Still determining domain or loading session
  if (isOnMainDomain === null || isSessionLoading) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
      </div>
    );
  }

  // Main domain - show org selection
  if (isOnMainDomain) {
    return (
      <>
        {orgStatus && statusMessages[orgStatus] && (
          <div className="fixed top-4 left-1/2 z-50 w-full max-w-md -translate-x-1/2 px-4">
            <Alert
              type={orgStatus === "pending" ? "info" : "error"}
              message={statusMessages[orgStatus]}
            />
          </div>
        )}
        <OrgSelection />
      </>
    );
  }

  // Subdomain with session - smart redirect
  if (session?.user) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
        <SmartRedirect
          onRedirect={(path) => {
            console.log(`Redirecting to ${path} based on user permissions`);
            router.push(path);
          }}
        />
      </div>
    );
  }

  // Loading state for subdomain without session (will redirect to /login)
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Loading />
    </div>
  );
}

export default function RootPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen flex-col items-center justify-center p-4">
          <Loading />
        </div>
      }
    >
      <RootPageContent />
    </Suspense>
  );
}
