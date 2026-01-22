"use client";

import { useEffect, useState } from "react";
import { useSession, isAdmin } from "~/lib/auth-client";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { BetterAuthInvitationCreateForm } from "~/components/admin/betterauth-invitation-create-form";
import { BetterAuthPendingInvitationsList } from "~/components/admin/betterauth-pending-invitations-list";
import { Loading } from "~/components/ui/loading";

export default function InvitationsPage() {
  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();

  const [refreshKey, setRefreshKey] = useState<number>(Date.now());
  const [isUserAdmin, setIsUserAdmin] = useState<boolean | null>(null);

  // Check admin status asynchronously via BetterAuth organization role
  useEffect(() => {
    if (session?.user) {
      void isAdmin().then(setIsUserAdmin);
    }
  }, [session]);

  if (isPending || isUserAdmin === null) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  // Redirect if not authenticated
  if (!session?.user) {
    redirect("/");
  }

  if (!isUserAdmin) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto max-w-2xl">
          <div className="rounded-2xl border border-red-200/50 bg-red-50/50 p-6">
            <div className="flex items-start gap-3">
              <svg
                className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <div>
                <h3 className="font-semibold text-red-900">
                  Keine Berechtigung
                </h3>
                <p className="mt-1 text-sm text-red-700">
                  Du verfügst nicht über die notwendigen Berechtigungen, um
                  Einladungen zu verwalten.
                </p>
              </div>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="space-y-6">
        <BetterAuthInvitationCreateForm
          onCreated={() => {
            setRefreshKey(Date.now());
          }}
        />
        <BetterAuthPendingInvitationsList refreshKey={refreshKey} />
      </div>
    </ResponsiveLayout>
  );
}
