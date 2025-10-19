"use client";

import { useState } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { isAdmin } from "~/lib/auth-utils";

export default function InvitationsPage() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const [refreshKey, setRefreshKey] = useState<number>(Date.now());

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-gray-200 border-t-[#5080d8]"></div>
      </div>
    );
  }

  if (!session || !isAdmin(session)) {
    return (
      <ResponsiveLayout>
        <div className="rounded-3xl border border-red-100 bg-red-50/70 p-6 text-sm text-red-700">
          Du verfügst nicht über die notwendigen Berechtigungen, um Einladungen zu verwalten.
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">Einladungen verwalten</h1>
        <p className="mt-1 text-sm text-gray-600">Sende neue Einladungen und behalte offene Einladungen im Blick.</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <InvitationForm
          onCreated={() => {
            setRefreshKey(Date.now());
          }}
        />
        <PendingInvitationsList refreshKey={refreshKey} />
      </div>
    </ResponsiveLayout>
  );
}
