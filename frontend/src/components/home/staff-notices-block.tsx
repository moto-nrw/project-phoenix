"use client";

import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import {
  HOME_CARD_BODY,
  HomeCardIcon,
} from "~/components/home/home-block-content";
import { fetchTodaysNotices } from "~/lib/staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  HomeCardLink,
  HomeMoreRow,
  useHomeCardRows,
} from "~/components/home/home-card-rows";

/**
 * Baustein „Tagesinformationen" der Startseite (#2180): die Hinweise der
 * Leitung, die heute gelten, an der Stelle, an der die Person morgens ohnehin
 * landet. Dieselbe Liste wie auf /tagesinformationen, damit eine Kenntnisnahme
 * hier genauso zählt wie dort.
 */
/**
 * Ein Hinweis ist ein Text mit Knopf, kein Listeneintrag: schon der zweite
 * ragt aus einer Karte dieser Höhe heraus (gemessen: ein Hinweis ~96 px,
 * Körper 142 px). Also einer ganz, der Rest als Zahl mit Weg.
 */
const MAX_NOTICES = 1;

export function StaffNoticesBlock() {
  const tenantPath = useTenantAwarePath();
  const {
    data: notices,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<StaffNotice[]>("staff-notices-today", fetchTodaysNotices, {
    revalidateOnFocus: false,
  });
  const { shown, hidden } = useHomeCardRows(notices ?? [], MAX_NOTICES);

  return (
    <SectionCard
      title="Tagesinformationen"
      // Die Karte füllt ihren Platz im Raster und scrollt in sich; ohne das
      // stünde sie kürzer als ihre Nachbarn und die Reihe wirkt kaputt.
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      leading={<HomeCardIcon concept="announcements" />}
      inlineActions
      actions={
        <HomeCardLink
          href={tenantPath("/tagesinformationen")}
          label="Tagesinformationen: alle ansehen"
        >
          Alle ansehen
        </HomeCardLink>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Die Tagesinformationen konnten nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && notices === undefined) {
          // Dieselbe Skelettform wie die Nachbarkarten. Ein kreisender Spinner
          // mitten in einer Reihe stiller Platzhalter zieht den Blick auf die
          // eine Karte, die gerade nichts zu sagen hat.
          return (
            <div className="space-y-2" aria-hidden="true">
              <div className="h-4 w-2/5 animate-pulse rounded bg-gray-200"></div>
              <div className="h-3 w-4/5 animate-pulse rounded bg-gray-200"></div>
              <div className="h-3 w-3/5 animate-pulse rounded bg-gray-200"></div>
            </div>
          );
        }
        if (!notices || notices.length === 0) {
          return (
            <EmptyState
              className="py-4"
              title="Heute gibt es keine Hinweise"
              description="Neue Hinweise der Leitung erscheinen hier."
            />
          );
        }
        return (
          <>
            <TodayNoticeList notices={shown} onChanged={mutate} />
            <HomeMoreRow
              hidden={hidden}
              href={tenantPath("/tagesinformationen")}
              label="Hinweise"
              singular="Hinweis"
            />
          </>
        );
      })()}
    </SectionCard>
  );
}
