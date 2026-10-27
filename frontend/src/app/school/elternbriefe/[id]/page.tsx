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
import { AnnouncementStatusBadge } from "~/components/announcements/announcement-meta";
import { BackLink } from "~/components/ui/back-link";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
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
import { SCHOOL_PARENT_LETTERS_ROUTE } from "~/lib/school-parent-letters";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";

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
  const refresh = async () => {
    await mutate();
  };

  const back = <BackLink href={listHref} label="Zurück zu den Elternbriefen" />;

  if (isLoading && !letter) {
    return (
      <div className="-mt-1.5 w-full space-y-6">
        {back}
        <SkeletonRegion label="Elternbrief wird geladen…">
          <ListSkeleton rows={4} avatar={false} />
        </SkeletonRegion>
      </div>
    );
  }

  if (!letter) {
    return (
      <div className="-mt-1.5 w-full space-y-6">
        {back}
        {notFound || !error ? (
          <p className="text-sm text-gray-600">
            Diesen Elternbrief gibt es nicht mehr.
          </p>
        ) : loadError ? (
          <LoadErrorAlert error={loadError} />
        ) : null}
      </div>
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
    <div className="-mt-1.5 w-full space-y-6">
      {back}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold break-words text-gray-900 sm:text-2xl">
            {letter.title}
          </h1>
          <p className="mt-1 text-sm text-gray-500">{statusLine}</p>
        </div>
        <div className="flex items-center gap-2">
          <AnnouncementStatusBadge status={letter.status} />
          {menuItems.length > 0 ? (
            <OverflowMenu ariaLabel="Weitere Aktionen" items={menuItems} />
          ) : null}
        </div>
      </div>

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
    </div>
  );
}
