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
  rotateParentCalendarFeed,
  type CalendarFeedInfo,
} from "~/lib/personal-calendar-api";

/**
 * Parents-portal panel to subscribe an external calendar (Apple/Google/Outlook)
 * to the family Termine. Once subscribed, new, changed and cancelled
 * appointments sync automatically — no per-event import needed.
 */
export function CalendarSubscribePanel() {
  const t = useTranslations("parentCalendarSubscribe");
  const toast = useToast();
  const [feed, setFeed] = useState<CalendarFeedInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setFeed(await getParentCalendarFeed());
      setOpen(true);
    } catch {
      toast.error(t("loadError"));
    } finally {
      setLoading(false);
    }
  };

  const rotate = async () => {
    setLoading(true);
    try {
      setFeed(await rotateParentCalendarFeed());
      toast.success(t("regenerated"));
    } catch {
      toast.error(t("rotateError"));
    } finally {
      setLoading(false);
    }
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t("copied"));
    } catch {
      toast.error(t("copyFailed"));
    }
  };

  return (
    <SectionCard
      icon={CalendarDays}
      title={t("title")}
      description={t("description")}
      bodyClassName="mt-4"
      actions={
        !open ? (
          <div className="flex w-full justify-end sm:w-auto">
            <Button type="button" size="md" isLoading={loading} onClick={load}>
              <Link className="mr-2 size-4" aria-hidden />
              {t("showLink")}
            </Button>
          </div>
        ) : feed && !feed.url ? (
          <Button type="button" size="md" isLoading={loading} onClick={rotate}>
            <RefreshCw className="mr-2 size-4" aria-hidden />
            {t("createNew")}
          </Button>
        ) : feed?.url ? (
          <>
            <ButtonLink href={feed.webcal_url} size="md">
              <CalendarDays className="mr-2 size-4" aria-hidden />
              {t("subscribe")}
            </ButtonLink>
            <Button
              type="button"
              size="md"
              variant="outline"
              isLoading={loading}
              onClick={rotate}
            >
              <RefreshCw className="mr-2 size-4" aria-hidden />
              {t("regenerate")}
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
                    id="calendar-feed-url"
                    label={t("linkLabel")}
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
                  onClick={() => copy(feed.url)}
                >
                  <Copy className="mr-2 size-4" aria-hidden />
                  {t("copy")}
                </Button>
              </div>
              <p className="mt-1 text-xs leading-5 text-gray-500">
                {t("linkOnce")}
              </p>
            </div>

            <div className="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
              <p className="font-semibold text-gray-700">{t("howToTitle")}</p>
              <p className="mt-1">{t("howToMac")}</p>
              <p className="mt-1">{t("howToApple")}</p>
              <p className="mt-1">{t("howToAndroid")}</p>
            </div>
          </div>
        ) : null}

        {open && feed && !feed.url ? (
          <div className="mt-4">
            <div className="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
              {t("alreadyActive")}
            </div>
          </div>
        ) : null}
      </div>
    </SectionCard>
  );
}
