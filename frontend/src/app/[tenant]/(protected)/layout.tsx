"use client";

import { TenantDataPrefetch } from "~/components/tenant/tenant-data-prefetch";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { SessionEndBarrier } from "~/components/auth/session-end-barrier";
import { GroupAttendanceCountProvider } from "~/lib/group-attendance-count-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { NotBookedPageProvider } from "~/lib/not-booked-page-context";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { AnnouncementModal } from "~/components/platform/announcement-modal";
import { PwaInstallHint } from "~/components/tenant/pwa-install-hint";
import { PortalNotificationSetup } from "~/components/notifications/portal-notification-setup";
import { SchoolSetupWizard } from "~/components/school-setup/school-setup-wizard";
import { StaffOnboardingWizard } from "~/components/staff-onboarding/staff-onboarding-wizard";
import { useSettingsCacheBridge } from "~/lib/hooks/use-settings-cache-bridge";

export default function ProtectedLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  useSettingsCacheBridge();
  return (
    <ShellIntlProvider>
      {/* Logouts of this shell stop late session answers (#5399). */}
      <SessionEndBarrier />
      <TeacherShellProvider>
        <BreadcrumbProvider>
          <GroupAttendanceCountProvider>
            <TenantDataPrefetch>
              <AppShell>
                {/* Seiten nicht gebuchter Module erklären sich im Gerüst (#4894). */}
                <NotBookedPageProvider>{children}</NotBookedPageProvider>
              </AppShell>
            </TenantDataPrefetch>
          </GroupAttendanceCountProvider>
          <AnnouncementModal />
          {/* Erste Schritte für neue Schulen (#2832). Nur für Personen, die
              die Einstellungen ändern dürfen, bis die Schule fertig ist. */}
          <SchoolSetupWizard />
          {/* Erste Schritte für Betreuungskräfte (#3748): alle, die die
              Einrichtung der Schule nicht sehen, sobald die Schule eine
              Gruppe oder ein Kind hat. */}
          <StaffOnboardingWizard />
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
