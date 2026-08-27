"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { StaffCardsSkeleton } from "./page-skeleton";

export default function StaffLoading() {
  return (
    <div className="w-full">
      {/* Titel und Kicker sind statisch, also rendert die echte Kopfkarte
          sofort; nur die Kartenliste darunter skelettiert. */}
      <PageIntro title="Mitarbeiter" className="mb-6">
        <PageHeaderWithSearch
          title=""
          search={{
            value: "",
            onChange: () => {},
            inputProps: { disabled: true },
          }}
        />
      </PageIntro>
      <StaffCardsSkeleton />
    </div>
  );
}
