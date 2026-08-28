"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { CalendarDays, Copy, Link, RefreshCw } from "lucide-react";

import { Button, ButtonLink } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useToast } from "~/contexts/ToastContext";
import {
  getParentCalendarFeed,
  getStaffCalendarFeed,
  rotateParentCalendarFeed,
  rotateStaffCalendarFeed,
  type CalendarFeedInfo,
} from "~/lib/personal-calendar-api";

interface CalendarSubscribeCopy {
  readonly title: string;
  readonly description: string;
  readonly showLink: string;
  readonly createNew: string;
  readonly subscribe: string;
  readonly regenerate: string;
  readonly linkLabel: string;
  readonly copy: string;
  readonly linkOnce: string;
  readonly howToTitle: string;
  readonly howToMac: string;
  readonly howToApple: string;
  readonly howToAndroid: string;
  readonly alreadyActive: string;
  readonly loadError: string;
  readonly rotateError: string;
  readonly regenerated: string;
  readonly copied: string;
  readonly copyFailed: string;
}

interface CalendarSubscribePanelViewProps {
  readonly copy: CalendarSubscribeCopy;
  readonly inputId: string;
  readonly loadFeed: () => Promise<CalendarFeedInfo>;
  readonly rotateFeed: () => Promise<CalendarFeedInfo>;
}

const staffCopy: CalendarSubscribeCopy = {
  title: "Kalender abonnieren",
  description:
    "Das Abo zeigt Ihre moto-Termine automatisch, aber Änderungen daran erreichen moto nicht.",
  showLink: "Abo-Link anzeigen",
  createNew: "Neuen Abo-Link erstellen",
  subscribe: "Im Kalender abonnieren",
  regenerate: "Link neu erstellen",
  linkLabel: "Abo-Link",
  copy: "Link kopieren",
  linkOnce:
    "Sie sehen diesen Link nur jetzt. Kopieren Sie ihn direkt in Ihren Kalender. Ein neuer Link beendet das bisherige Abo.",
  howToTitle: "So geht es:",
  howToMac:
    "Mac: Wählen Sie „Im Kalender abonnieren“. Apple Kalender öffnet den Link.",
  howToApple:
    "iPhone und iPad: Kopieren Sie den Link. Öffnen Sie Einstellungen > Apps > Kalender > Kalenderaccounts. Wählen Sie Account hinzufügen > Andere > Kalenderabo hinzufügen. Fügen Sie den Link ein.",
  howToAndroid:
    "Google Kalender und Outlook: Kopieren Sie den Link. Fügen Sie ihn als Kalender aus dem Internet hinzu.",
  alreadyActive:
    "Ihr Abo läuft bereits. Aus Sicherheitsgründen sehen Sie den Link nicht erneut. Ein neuer Link beendet das bisherige Abo.",
  loadError:
    "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
  rotateError:
    "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
  regenerated: "Neuer Abo-Link erstellt. Der alte Link gilt nicht mehr.",
  copied: "Abo-Link kopiert.",
  copyFailed:
    "Das Kopieren hat leider nicht geklappt. Bitte kopieren Sie den Link von Hand.",
};

/**
 * Parents-portal panel to subscribe an external calendar (Apple/Google/Outlook)
 * to the family Termine. Once subscribed, new, changed and cancelled
 * appointments sync automatically — no per-event import needed.
 */
export function CalendarSubscribePanel() {
  const t = useTranslations("parentCalendarSubscribe");
  const copy: CalendarSubscribeCopy = {
    title: t("title"),
    description: t("description"),
    showLink: t("showLink"),
    createNew: t("createNew"),
    subscribe: t("subscribe"),
    regenerate: t("regenerate"),
    linkLabel: t("linkLabel"),
    copy: t("copy"),
    linkOnce: t("linkOnce"),
    howToTitle: t("howToTitle"),
    howToMac: t("howToMac"),
    howToApple: t("howToApple"),
    howToAndroid: t("howToAndroid"),
    alreadyActive: t("alreadyActive"),
    loadError: t("loadError"),
    rotateError: t("rotateError"),
    regenerated: t("regenerated"),
    copied: t("copied"),
    copyFailed: t("copyFailed"),
  };

  return (
    <CalendarSubscribePanelView
      copy={copy}
      inputId="parent-calendar-feed-url"
      loadFeed={getParentCalendarFeed}
      rotateFeed={rotateParentCalendarFeed}
    />
  );
}

export function StaffCalendarSubscribePanel() {
  return (
    <CalendarSubscribePanelView
      copy={staffCopy}
      inputId="staff-calendar-feed-url"
      loadFeed={getStaffCalendarFeed}
      rotateFeed={rotateStaffCalendarFeed}
    />
  );
}

function CalendarSubscribePanelView({
  copy,
  inputId,
  loadFeed,
  rotateFeed,
}: CalendarSubscribePanelViewProps) {
  const toast = useToast();
  const [feed, setFeed] = useState<CalendarFeedInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setFeed(await loadFeed());
      setOpen(true);
    } catch {
      toast.error(copy.loadError);
    } finally {
      setLoading(false);
    }
  };

  const rotate = async () => {
    setLoading(true);
    try {
      setFeed(await rotateFeed());
      toast.success(copy.regenerated);
    } catch {
      toast.error(copy.rotateError);
    } finally {
      setLoading(false);
    }
  };

  const copyLink = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(copy.copied);
    } catch {
      toast.error(copy.copyFailed);
    }
  };

  return (
    <SectionCard
      icon={CalendarDays}
      title={copy.title}
      description={copy.description}
      bodyClassName="mt-4"
      actions={
        !open ? (
          <div className="flex w-full justify-end sm:w-auto">
            <Button type="button" size="md" isLoading={loading} onClick={load}>
              <Link className="mr-2 size-4" aria-hidden />
              {copy.showLink}
            </Button>
          </div>
        ) : feed && !feed.url ? (
          <Button type="button" size="md" isLoading={loading} onClick={rotate}>
            <RefreshCw className="mr-2 size-4" aria-hidden />
            {copy.createNew}
          </Button>
        ) : feed?.url ? (
          <>
            <ButtonLink href={feed.webcal_url} size="md">
              <CalendarDays className="mr-2 size-4" aria-hidden />
              {copy.subscribe}
            </ButtonLink>
            <Button
              type="button"
              size="md"
              variant="outline"
              isLoading={loading}
              onClick={rotate}
            >
              <RefreshCw className="mr-2 size-4" aria-hidden />
              {copy.regenerate}
            </Button>
          </>
        ) : undefined
      }
    >
      <div className="min-w-0">
        {open && feed && feed.url ? (
          <div className="mt-4 space-y-4">
            <div>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
                <div className="min-w-0 flex-1">
                  <Input
                    id={inputId}
                    label={copy.linkLabel}
                    readOnly
                    value={feed.url}
                    controlSize="compact"
                    onFocus={(event) => event.target.select()}
                    className="bg-gray-50 text-gray-700"
                  />
                </div>
                <Button
                  type="button"
                  size="md"
                  variant="outline"
                  className="shrink-0"
                  onClick={() => copyLink(feed.url)}
                >
                  <Copy className="mr-2 size-4" aria-hidden />
                  {copy.copy}
                </Button>
              </div>
              <p className="mt-1 text-xs leading-5 text-gray-500">
                {copy.linkOnce}
              </p>
            </div>

            <div className="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
              <p className="font-semibold text-gray-700">{copy.howToTitle}</p>
              <p className="mt-1">{copy.howToMac}</p>
              <p className="mt-1">{copy.howToApple}</p>
              <p className="mt-1">{copy.howToAndroid}</p>
            </div>
          </div>
        ) : null}

        {open && feed && !feed.url ? (
          <div className="mt-4">
            <div className="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
              {copy.alreadyActive}
            </div>
          </div>
        ) : null}
      </div>
    </SectionCard>
  );
}
