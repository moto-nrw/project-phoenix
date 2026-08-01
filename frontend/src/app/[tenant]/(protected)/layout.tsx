"use client";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { GroupAttendanceCountProvider } from "~/lib/group-attendance-count-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { ShellNavIntlProvider } from "~/components/dashboard/shell-nav-intl-provider";
import { AnnouncementModal } from "~/components/platform/announcement-modal";
import { PwaInstallHint } from "~/components/tenant/pwa-install-hint";
import { useSettingsCacheBridge } from "~/lib/hooks/use-settings-cache-bridge";

export default function ProtectedLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  useSettingsCacheBridge();
  return (
    <TeacherShellProvider>
      <BreadcrumbProvider>
        <GroupAttendanceCountProvider>
          <ShellNavIntlProvider>
            <AppShell>{children}</AppShell>
          </ShellNavIntlProvider>
        </GroupAttendanceCountProvider>
        <AnnouncementModal />
        {/* Layout-level on purpose: the install promotion targets staff on
            phones and tablets, and most of them never reach the admin-only
            dashboard. */}
        <PwaInstallHint />
      </BreadcrumbProvider>
    </TeacherShellProvider>
  );
}
