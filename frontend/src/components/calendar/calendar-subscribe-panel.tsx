"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  CalendarDays,
  Check,
  Copy,
  KeyRound,
  Link,
  RefreshCw,
} from "lucide-react";

import { Button, ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { useToast } from "~/contexts/ToastContext";
import { getApiErrorMessage } from "~/lib/api-error-message";
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
  readonly calDAVEnabled?: boolean;
}

interface CopyValueRowProps {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly copyLabel: string;
  readonly copiedLabel: string;
  readonly copiedMessage: string;
  readonly copyFailedMessage: string;
  readonly secret?: boolean;
}

function CopyValueRow({
  id,
  label,
  value,
  copyLabel,
  copiedLabel,
  copiedMessage,
  copyFailedMessage,
  secret = false,
}: CopyValueRowProps) {
  const toast = useToast();
  const { copied, copy } = useClipboardCopy(
    `CalendarSubscribePanel-${id}`,
    2000,
  );

  const copyValue = async () => {
    if (await copy(value)) {
      toast.success(copiedMessage);
    } else {
      toast.error(copyFailedMessage);
    }
  };

  return (
    <div className="rounded-xl bg-gray-50 p-3">
      <p className="text-xs font-medium text-gray-500">{label}</p>
      <div className="mt-1 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p
          id={id}
          className={`min-w-0 text-sm break-all text-gray-800 ${secret ? "font-mono" : ""}`}
        >
          {value}
        </p>
        <Button
          type="button"
          size="sm"
          variant={copied ? "success" : "outline"}
          className="shrink-0 sm:min-w-28"
          aria-describedby={id}
          onClick={copyValue}
        >
          {copied ? (
            <Check className="mr-2 size-4" aria-hidden />
          ) : (
            <Copy className="mr-2 size-4" aria-hidden />
          )}
          {copied ? copiedLabel : copyLabel}
        </Button>
      </div>
    </div>
  );
}

/**
 * Shared panel for subscribing an external calendar to parent or staff
 * appointments. The audience selects only the API and audience-specific copy.
 */
export function CalendarSubscribePanel({
  audience = "parent",
  calDAVEnabled = false,
}: CalendarSubscribePanelProps) {
  const t = useTranslations("parentCalendarSubscribe");
  const staffT = useTranslations("staffCalendarSubscribe");
  const isStaff = audience === "staff";
  const copy: CalendarSubscribeCopy = {
    title: t("title"),
    description: isStaff ? staffT("description") : t("description"),
    showLink: t("showLink"),
    createNew: isStaff ? staffT("createNew") : t("createNew"),
    subscribe: t("subscribe"),
    regenerate: isStaff ? staffT("regenerate") : t("regenerate"),
    linkLabel: t("linkLabel"),
    copy: t("copy"),
    copiedButton: t("copiedButton"),
    linkOnce: t("linkOnce"),
    howToTitle: t("howToTitle"),
    howToMac: t("howToMac"),
    howToApple: t("howToApple"),
    howToAndroid: t("howToAndroid"),
    alreadyActive: isStaff ? staffT("alreadyActive") : t("alreadyActive"),
    loadError: t("loadError"),
    rotateError: t("rotateError"),
    regenerated: isStaff ? staffT("regenerated") : t("regenerated"),
    copied: t("copied"),
    copyFailed: t("copyFailed"),
  };
  const loadFeed = isStaff ? getStaffCalendarFeed : getParentCalendarFeed;
  const rotateFeed = isStaff
    ? rotateStaffCalendarFeed
    : rotateParentCalendarFeed;
  const toast = useToast();
  const [feed, setFeed] = useState<CalendarFeedInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setFeed(await loadFeed());
      setOpen(true);
    } catch (err) {
      // Showing the link creates it on first use, so the read-only staff
      // preview blocks this call — say that instead of a generic failure.
      toast.error(
        getApiErrorMessage(
          err,
          "anzeigen",
          "den Kalender-Link",
          copy.loadError,
        ),
      );
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
              <CopyValueRow
                id={
                  isStaff
                    ? "staff-calendar-feed-url"
                    : "parent-calendar-feed-url"
                }
                label={copy.linkLabel}
                value={feed.url}
                copyLabel={copy.copy}
                copiedLabel={copy.copiedButton}
                copiedMessage={copy.copied}
                copyFailedMessage={copy.copyFailed}
              />
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

        {open && isStaff && calDAVEnabled ? (
          <div className="mt-6 border-t border-gray-200 pt-5">
            <div className="flex items-start gap-3">
              <KeyRound
                className="text-moto-blue mt-0.5 size-5 shrink-0"
                aria-hidden
              />
              <div>
                <h3 className="font-semibold text-gray-900">
                  {staffT("caldavTitle")}
                </h3>
                <p className="mt-1 text-sm leading-6 text-gray-600">
                  {staffT("caldavDescription")}
                </p>
              </div>
            </div>

            {feed?.caldav ? (
              <div className="mt-4 space-y-3">
                <CopyValueRow
                  id="staff-caldav-server-url"
                  label={staffT("caldavAddress")}
                  value={feed.caldav.server_url}
                  copyLabel={copy.copy}
                  copiedLabel={copy.copiedButton}
                  copiedMessage={staffT("addressCopied")}
                  copyFailedMessage={copy.copyFailed}
                />
                <CopyValueRow
                  id="staff-caldav-username"
                  label={staffT("caldavUsername")}
                  value={feed.caldav.username}
                  copyLabel={copy.copy}
                  copiedLabel={copy.copiedButton}
                  copiedMessage={staffT("usernameCopied")}
                  copyFailedMessage={copy.copyFailed}
                />
                {feed.caldav.app_password ? (
                  <>
                    <CopyValueRow
                      id="staff-caldav-app-password"
                      label={staffT("caldavPassword")}
                      value={feed.caldav.app_password}
                      copyLabel={copy.copy}
                      copiedLabel={copy.copiedButton}
                      copiedMessage={staffT("passwordCopied")}
                      copyFailedMessage={copy.copyFailed}
                      secret
                    />
                    <p className="text-xs leading-5 text-gray-500">
                      {staffT("passwordOnce")}
                    </p>
                  </>
                ) : (
                  <p className="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
                    {staffT("passwordHidden")}
                  </p>
                )}
              </div>
            ) : (
              <p className="mt-4 rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600">
                {staffT("caldavUnavailable")}
              </p>
            )}
          </div>
        ) : null}
      </div>
    </SectionCard>
  );
}
