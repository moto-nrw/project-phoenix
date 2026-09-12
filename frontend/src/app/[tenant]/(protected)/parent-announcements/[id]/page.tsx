"use client";

import { Suspense, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui/alert";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { AnnouncementDetail } from "~/components/announcements/announcement-detail";
import {
  buildAnnouncementMenuItems,
  DeleteAnnouncementDialog,
  PublishAnnouncementDialog,
  UnpublishAnnouncementDialog,
} from "~/components/announcements/announcement-lifecycle-dialogs";
import {
  announcementCollectionPath,
  AnnouncementStatusBadge,
  kindOf,
  KIND_PARAM,
} from "~/components/announcements/announcement-meta";
import type { AnnouncementKind } from "~/components/announcements/announcement-meta";
import { groupService } from "~/lib/api";
import type { Group } from "~/lib/api";
import { fetchActivities } from "~/lib/activity-api";
import type { Activity } from "~/lib/activity-helpers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { fetchAnnouncement } from "~/lib/parent-announcements-api";
import type { Announcement } from "~/lib/parent-announcements-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { AnnouncementDetailLoadingPage } from "./page-skeleton";

/** SWR-Schlüssel der Liste, die nach jeder Lebenszyklus-Aktion neu lädt. */
const LIST_KEY = "parent-announcements-list";

function announcementDetailKey(id: string): string {
  return `parent-announcement-${id}`;
}

const BACK_LABEL: Record<AnnouncementKind, string> = {
  announcement: "Zurück zu den Mitteilungen",
  letter: "Zurück zu den Elternbriefen",
  poll: "Zurück zu den Umfragen",
};

type LifecycleAction = "publish" | "unpublish" | "delete" | null;

export default function AnnouncementDetailPage() {
  return (
    <Suspense fallback={<AnnouncementDetailLoadingPage />}>
      <AnnouncementDetailPageContent />
    </Suspense>
  );
}

/**
 * Die Objektseite einer Elternmitteilung (BAUARTEN-SPEC Bauart 2, #3115):
 * vorher ein Slide-over über der Liste, jetzt eine Adresse, die sich teilen,
 * neu laden und per Zurück verlassen lässt. Der Assistent zum Anlegen und
 * Bearbeiten bleibt auf der Liste; „Bearbeiten" führt mit `?bearbeiten=`
 * dorthin.
 */
function AnnouncementDetailPageContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const tenantMutate = useTenantMutate();
  const announcementId = params.id as string;

  const {
    data: announcement,
    isLoading,
    error: loadError,
    mutate,
  } = useSWRAuth<Announcement | undefined>(
    announcementDetailKey(announcementId),
    () => fetchAnnouncement(announcementId),
    { revalidateOnFocus: false },
  );

  // Der Rückweg: die Sammlung samt Reiter, aus der die Seite geöffnet wurde.
  // Ohne `from` (Link von außen) führt er auf den Reiter, zu dem die
  // Mitteilung gehört.
  const kind = announcement ? kindOf(announcement) : "announcement";
  const referrer = searchParams.get("from") ?? announcementCollectionPath(kind);
  const backLabel = BACK_LABEL[kind];

  useSetBreadcrumb({
    announcementTitle: announcement?.title,
    referrerPage: referrer,
  });

  // Die Zielgruppen tragen echte Namen; die Nachschlagelisten laden erst,
  // wenn die Mitteilung da ist.
  const { data: groups } = useSWRAuth<Group[]>(
    announcement ? "parent-announcements-groups" : null,
    () => groupService.getGroups(),
    { revalidateOnFocus: false },
  );
  const { data: activities } = useSWRAuth<Activity[]>(
    announcement ? "parent-announcements-activities" : null,
    () => fetchActivities(),
    { revalidateOnFocus: false },
  );

  const [action, setAction] = useState<LifecycleAction>(null);
  const [reminderNotice, setReminderNotice] = useState("");

  const refresh = async () => {
    await Promise.all([mutate(), tenantMutate(LIST_KEY)]);
  };

  if (isLoading && !announcement) {
    return (
      <AnnouncementDetailLoadingPage
        referrer={referrer}
        backLabel={backLabel}
      />
    );
  }

  if (loadError || !announcement) {
    return (
      <TenantPage
        title="Mitteilung"
        back
        backHref={referrer}
        backLabel={backLabel}
        error={
          loadError
            ? "Elternmitteilung konnte nicht geladen werden."
            : "Elternmitteilung nicht gefunden."
        }
      />
    );
  }

  const menuItems = buildAnnouncementMenuItems(announcement, {
    onPublish: () => setAction("publish"),
    onEdit: () =>
      router.push(
        `/parent-announcements?art=${KIND_PARAM[kind]}&bearbeiten=${encodeURIComponent(announcement.id)}`,
      ),
    onUnpublish: () => setAction("unpublish"),
    onDelete: () => setAction("delete"),
  });

  const statusLine = [
    announcement.published_at
      ? `Veröffentlicht ${formatDate(announcement.published_at)}`
      : "Noch nicht veröffentlicht",
    announcement.response_deadline
      ? `Antwort bis ${formatBerlinDate(announcement.response_deadline)}`
      : announcement.expires_at
        ? `Läuft ab ${formatBerlinDate(announcement.expires_at)}`
        : null,
  ]
    .filter(Boolean)
    .join(" · ");

  const concept =
    kind === "poll" ? MOTO_CONCEPTS.polls : MOTO_CONCEPTS.announcements;

  return (
    <TenantPage
      title={announcement.title}
      stats={statusLine}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={
        <MotoDuotoneIcon icon={concept.icon} tone={concept.tone} size={40} />
      }
      actions={
        <>
          <AnnouncementStatusBadge status={announcement.status} />
          {announcement.system_kind === "care_cancellation" ? (
            <StatusBadge
              label="Ausfall"
              tone="red"
              title="Automatisch beim Absagen eines Termins erstellt"
            />
          ) : announcement.priority === "important" ? (
            <StatusBadge label="Wichtig" tone="gray" />
          ) : null}
          {menuItems.length > 0 ? (
            <OverflowMenu ariaLabel="Weitere Aktionen" items={menuItems} />
          ) : null}
        </>
      }
      overlays={
        <>
          {action === "publish" && (
            <PublishAnnouncementDialog
              announcement={announcement}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "unpublish" && (
            <UnpublishAnnouncementDialog
              announcement={announcement}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "delete" && (
            <DeleteAnnouncementDialog
              announcement={announcement}
              onClose={() => setAction(null)}
              onDone={async () => {
                await tenantMutate(LIST_KEY);
                router.push(referrer);
              }}
            />
          )}
        </>
      }
    >
      {reminderNotice && <Alert type="success" message={reminderNotice} />}
      <AnnouncementDetail
        announcement={announcement}
        groups={groups ?? []}
        activities={activities ?? []}
        onReminded={(count) =>
          setReminderNotice(
            count === 0
              ? "Alle erreichten Kinder haben bereits geantwortet, es wurde niemand erinnert."
              : `${count} ${count === 1 ? "Elternteil wurde" : "Eltern wurden"} an die offene Umfrage erinnert.`,
          )
        }
      />
    </TenantPage>
  );
}
