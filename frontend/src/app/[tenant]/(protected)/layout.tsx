"use client";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { GroupAttendanceCountProvider } from "~/lib/group-attendance-count-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { ShellNavIntlProvider } from "~/components/dashboard/shell-nav-intl-provider";
import { AnnouncementModal } from "~/components/platform/announcement-modal";
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
      </BreadcrumbProvider>
    </TeacherShellProvider>
  );
}
