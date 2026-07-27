"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import {
  GradeTransitionsManager,
  type TransitionPermissions,
} from "~/components/database/grade-transitions/grade-transitions-manager";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { hasPermission } from "~/lib/auth-utils";

export default function GradeTransitionsPage() {
  const { data: session, status } = useSession();

  const permissions = useMemo<TransitionPermissions>(
    () => ({
      canCreate: hasPermission(session, "grade_transitions:create"),
      canUpdate: hasPermission(session, "grade_transitions:update"),
      canDelete: hasPermission(session, "grade_transitions:delete"),
      canApply: hasPermission(session, "grade_transitions:apply"),
      canPurge: hasPermission(session, "users:delete"),
    }),
    [session],
  );

  const canRead = hasPermission(session, "grade_transitions:read");

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Jahrgangswechsel" />
        {status === "loading" ? null : canRead ? (
          <GradeTransitionsManager permissions={permissions} />
        ) : (
          <p className="text-sm text-gray-600">
            Sie haben keine Berechtigung, Jahrgangswechsel anzusehen.
          </p>
        )}
      </div>
    </div>
  );
}
