"use client";

// Ein eigener Elternbrief in moto schule (#4800): der Text, die Zustellung
// und der Stand, wer gelesen, bestätigt oder zugestimmt hat. Dieselbe
// Ansicht wie im OGS-Portal, über die Routen von moto schule. Andere Briefe
// als die eigenen gibt das Backend hier nicht heraus.

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useSession } from "next-auth/react";

import { AnnouncementDetail } from "~/components/announcements/announcement-detail";
import {
  buildAnnouncementMenuItems,
  DeleteAnnouncementDialog,
  PublishAnnouncementDialog,
  UnpublishAnnouncementDialog,
} from "~/components/announcements/announcement-lifecycle-dialogs";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage } from "~/components/ui/tenant-page";
import { useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  fetchAnnouncement,
  SCHOOL_PARENT_LETTERS_BASE,
  type Announcement,
} from "~/lib/parent-announcements-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  SCHOOL_PARENT_LETTER_STATUS,
  SCHOOL_PARENT_LETTERS_ROUTE,
} from "~/lib/school-parent-letters";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { StatusBadge } from "~/components/ui/status-badge";

type LifecycleAction = "publish" | "unpublish" | "delete" | null;

export default function SchoolParentLetterPage() {
  const params = useParams();
  const letterId = params.id as string;
  const router = useRouter();
  const { data: session } = useSession();
  const { success: toastSuccess } = useToast();
  const [action, setAction] = useState<LifecycleAction>(null);

  const {
    data: letter,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<Announcement | undefined>(
    session ? `school-parent-letter-${letterId}` : null,
    () => fetchAnnouncement(letterId, SCHOOL_PARENT_LETTERS_BASE),
    { revalidateOnFocus: false },
  );
  const notFound = error instanceof ApiError && error.status === 404;
  const loadError = useSwrLoadError(
    notFound ? undefined : error,
    "den Elternbrief",
    () => void mutate(),
  );

  const listHref = schoolPath(SCHOOL_PARENT_LETTERS_ROUTE);
  // Die Liste lädt beim Öffnen neu; hier genügt der Brief selbst.
  const refresh = () => mutate().then(() => undefined);

  const back = {
    back: true,
    backHref: listHref,
    backLabel: "Zurück zu den Elternbriefen",
  } as const;

  if (!letter) {
    const gone = notFound || (!isLoading && !error);
    return (
      <TenantPage
        title="Elternbrief"
        {...back}
        loading={isLoading}
        loadingLabel="Elternbrief wird geladen…"
        error={gone ? null : loadError}
        empty={
          gone ? { title: "Diesen Elternbrief gibt es nicht mehr." } : null
        }
      />
    );
  }

  const menuItems = buildAnnouncementMenuItems(letter, {
    classLetterAuthor: true,
    onPublish: () => setAction("publish"),
    onUnpublish: () => setAction("unpublish"),
    onDelete: () => setAction("delete"),
  });
  const statusLine = [
    schoolClassLabel(letter.sender_school_class ?? ""),
    letter.published_at
      ? `Gesendet ${formatDate(letter.published_at)}`
      : "Noch nicht gesendet",
  ].join(" · ");

  return (
    <TenantPage
      title={letter.title}
      {...back}
      stats={statusLine}
      actions={
        <>
          <StatusBadge {...SCHOOL_PARENT_LETTER_STATUS[letter.status]} />
          {menuItems.length > 0 ? (
            <OverflowMenu ariaLabel="Weitere Aktionen" items={menuItems} />
          ) : null}
        </>
      }
      overlays={
        <>
          {action === "publish" && (
            <PublishAnnouncementDialog
              announcement={letter}
              apiBase={SCHOOL_PARENT_LETTERS_BASE}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "unpublish" && (
            <UnpublishAnnouncementDialog
              announcement={letter}
              apiBase={SCHOOL_PARENT_LETTERS_BASE}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "delete" && (
            <DeleteAnnouncementDialog
              announcement={letter}
              apiBase={SCHOOL_PARENT_LETTERS_BASE}
              onClose={() => setAction(null)}
              onDone={() => router.push(listHref)}
            />
          )}
        </>
      }
    >
      <AnnouncementDetail
        announcement={letter}
        groups={[]}
        activities={[]}
        apiBase={SCHOOL_PARENT_LETTERS_BASE}
        onReminded={(count) =>
          toastSuccess(
            count === 0
              ? "Alle Eltern haben schon geantwortet. Es wurde niemand erinnert."
              : `${count} ${count === 1 ? "Elternteil wurde" : "Eltern wurden"} erinnert.`,
          )
        }
      />
    </TenantPage>
  );
}
