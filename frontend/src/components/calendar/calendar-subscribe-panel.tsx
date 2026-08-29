"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { CalendarDays, Check, Copy, Link, RefreshCw } from "lucide-react";

import { Button, ButtonLink } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useToast } from "~/contexts/ToastContext";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";
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
  readonly copiedButton: string;
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

interface CalendarSubscribePanelProps {
  readonly audience?: "parent" | "staff";
}

/**
 * Shared panel for subscribing an external calendar to parent or staff
 * appointments. The audience selects only the API and audience-specific copy.
 */
export function CalendarSubscribePanel({
  audience = "parent",
}: CalendarSubscribePanelProps) {
  const t = useTranslations("parentCalendarSubscribe");
  const staffT = useTranslations("staffCalendarSubscribe");
  const isStaff = audience === "staff";
  const copy: CalendarSubscribeCopy = {
    title: t("title"),
    description: isStaff ? staffT("description") : t("description"),
    showLink: t("showLink"),
    createNew: t("createNew"),
    subscribe: t("subscribe"),
    regenerate: t("regenerate"),
    linkLabel: t("linkLabel"),
    copy: t("copy"),
    copiedButton: t("copiedButton"),
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
  const inputId = isStaff
    ? "staff-calendar-feed-url"
    : "parent-calendar-feed-url";
  const loadFeed = isStaff ? getStaffCalendarFeed : getParentCalendarFeed;
  const rotateFeed = isStaff
    ? rotateStaffCalendarFeed
    : rotateParentCalendarFeed;
  const toast = useToast();
  const [feed, setFeed] = useState<CalendarFeedInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const { copied, copy: copyToClipboard } = useClipboardCopy(
    "CalendarSubscribePanel",
    2000,
  );

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
    if (await copyToClipboard(text)) {
      toast.success(copy.copied);
    } else {
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
                  variant={copied ? "success" : "outline"}
                  className="min-w-36 shrink-0"
                  onClick={() => copyLink(feed.url)}
                >
                  {copied ? (
                    <Check className="mr-2 size-4" aria-hidden />
                  ) : (
                    <Copy className="mr-2 size-4" aria-hidden />
                  )}
                  {copied ? copy.copiedButton : copy.copy}
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
