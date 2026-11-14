"use client";

import { useEffect } from "react";
import { useParams, usePathname } from "next/navigation";
import { ParentAttentionDialog } from "~/components/students/parent-attention-dialog";
import { useParentAttentionOverlay } from "~/lib/parent-attention-overlay";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";

export default function ParentAttentionOverlayPage() {
  const { studentId } = useParams<{ studentId: string }>();
  const { selection } = useParentAttentionOverlay();
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const active =
    usePathname() ===
    tenantPath(`/parent-attention/${encodeURIComponent(studentId)}`);
  const current = selection?.studentId === studentId ? selection : null;
  useEffect(() => {
    if (active && !current)
      router.replace(`/students/${encodeURIComponent(studentId)}`);
  }, [active, current, router, studentId]);
  if (!active || !current) return null;
  return (
    <ParentAttentionDialog {...current} isOpen onClose={() => router.back()} />
  );
}
