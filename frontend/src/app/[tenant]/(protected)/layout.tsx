"use client";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { GroupAttendanceCountProvider } from "~/lib/group-attendance-count-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { AnnouncementModal } from "~/components/platform/announcement-modal";
import { PwaInstallHint } from "~/components/tenant/pwa-install-hint";
import { PortalNotificationSetup } from "~/components/notifications/portal-notification-setup";
import { useSettingsCacheBridge } from "~/lib/hooks/use-settings-cache-bridge";

export default function ProtectedLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  useSettingsCacheBridge();
  return (
    <ShellIntlProvider>
      <TeacherShellProvider>
        <BreadcrumbProvider>
          <GroupAttendanceCountProvider>
            <AppShell>{children}</AppShell>
          </GroupAttendanceCountProvider>
          <AnnouncementModal />
          {/* Samsung Internet needs its own route to Chrome. All other mobile
              installation guidance belongs to the notification setup. */}
          <PwaInstallHint samsungOnly />
          {/* Derselbe geführte Einstieg wie im Elternportal (#2831). Ohne ihn
              findet eine Betreuungskraft die Einrichtung nur zufällig. */}
          <PortalNotificationSetup portal="tenant" />
        </BreadcrumbProvider>
      </TeacherShellProvider>
    </ShellIntlProvider>
  );
}
