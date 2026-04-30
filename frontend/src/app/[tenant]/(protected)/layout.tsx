"use client";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { GroupAttendanceCountProvider } from "~/lib/group-attendance-count-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { AnnouncementModal } from "~/components/platform/announcement-modal";

export default function ProtectedLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <TeacherShellProvider>
      <BreadcrumbProvider>
        <GroupAttendanceCountProvider>
          <AppShell>{children}</AppShell>
        </GroupAttendanceCountProvider>
        <AnnouncementModal />
      </BreadcrumbProvider>
    </TeacherShellProvider>
  );
}
