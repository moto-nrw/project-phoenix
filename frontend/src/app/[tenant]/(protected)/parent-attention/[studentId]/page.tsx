"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";

/** A reload has no card selection; continue in the child's existing record. */
export default function ParentAttentionRedirect() {
  const { studentId } = useParams<{ studentId: string }>();
  const router = useTenantRouter();
  useEffect(() => {
    router.replace(`/students/${encodeURIComponent(studentId)}`);
  }, [router, studentId]);
  return null;
}
