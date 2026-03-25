"use client";

import { useState } from "react";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { RoleGuard } from "~/components/auth/role-guard";

function InvitationsContent() {
  const [refreshKey, setRefreshKey] = useState<number>(Date.now());

  return (
    <div className="space-y-6">
      <InvitationForm
        onCreated={() => {
          setRefreshKey(Date.now());
        }}
      />
      <PendingInvitationsList refreshKey={refreshKey} />
    </div>
  );
}

export default function InvitationsPage() {
  return (
    <RoleGuard
      variant="adminOnly"
      message="Du verfügst nicht über die notwendigen Berechtigungen, um Einladungen zu verwalten."
    >
      <InvitationsContent />
    </RoleGuard>
  );
}
