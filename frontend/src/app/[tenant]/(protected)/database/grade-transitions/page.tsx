"use client";

import { GradeTransitionsManager } from "~/components/database/grade-transitions/grade-transitions-manager";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";

export default function GradeTransitionsPage() {
  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Jahrgangswechsel" />
        <GradeTransitionsManager />
      </div>
    </div>
  );
}
