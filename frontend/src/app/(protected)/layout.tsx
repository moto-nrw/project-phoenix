"use client";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { AppShell } from "~/components/dashboard/app-shell";

export default function ProtectedLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <BreadcrumbProvider>
      <AppShell>{children}</AppShell>
    </BreadcrumbProvider>
  );
}
