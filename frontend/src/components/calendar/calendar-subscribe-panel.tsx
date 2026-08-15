"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Copy, Link, RefreshCw } from "lucide-react";

import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
    <div className="moto-content-surface rounded-2xl border border-gray-200 p-4 shadow-sm sm:p-6">
      <div className="flex items-start gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          <MotoConceptIcon concept="calendar" size={22} />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="text-[20px] font-semibold text-gray-900">
            {t("title")}
          </h2>
          <p className="mt-1 text-[15px] leading-6 text-gray-600">
            {t("description")}
          </p>

          {!open ? (
            <Button
              type="button"
              size="touch"
              className="mt-4"
              isLoading={loading}
              onClick={load}
            >
              <Link className="mr-2 h-5 w-5" aria-hidden />
              {t("showLink")}
            </Button>
          ) : null}

          {open && feed && feed.url ? (
            <div className="mt-4 space-y-4">
              <a
                href={feed.webcal_url}
                className="bg-moto-green hover:bg-moto-green-hover inline-flex min-h-12 items-center justify-center gap-2 rounded-xl px-5 text-[17px] font-semibold text-gray-950 transition-colors"
              >
                <MotoConceptIcon concept="calendar" size={20} />
                {t("subscribe")}
              </a>

              <div>
                <label
                  htmlFor="calendar-feed-url"
                  className="mb-1 block text-[15px] font-medium text-gray-600"
                >
                  {t("linkLabel")}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    id="calendar-feed-url"
                    readOnly
                    value={feed.url}
                    onFocus={(event) => event.target.select()}
                    className="h-12 w-full rounded-md border-0 bg-gray-50 px-3 text-[15px] text-gray-700 ring-1 ring-gray-200 ring-inset"
                  />
                  <Button
                    type="button"
                    size="touch"
                    variant="outline"
                    onClick={() => copy(feed.url)}
                  >
                    <Copy className="mr-2 h-5 w-5" aria-hidden />
                    {t("copy")}
                  </Button>
                </div>
                <p className="mt-1 text-[15px] text-gray-500">
                  {t("linkOnce")}
                </p>
              </div>

              <div className="rounded-xl bg-gray-50 p-3 text-[15px] leading-6 text-gray-600">
                <p className="font-semibold text-gray-700">{t("howToTitle")}</p>
                <p className="mt-1">{t("howToApple")}</p>
                <p className="mt-1">{t("howToAndroid")}</p>
              </div>

              <Button
                type="button"
                size="touch"
                variant="ghost"
                className="text-gray-600"
                isLoading={loading}
                onClick={rotate}
              >
                <RefreshCw className="mr-2 h-5 w-5" aria-hidden />
                {t("regenerate")}
              </Button>
            </div>
          ) : null}

          {open && feed && !feed.url ? (
            <div className="mt-4 space-y-3">
              <div className="rounded-xl bg-gray-50 p-3 text-[15px] leading-6 text-gray-600">
                {t("alreadyActive")}
              </div>
              <Button
                type="button"
                size="touch"
                isLoading={loading}
                onClick={rotate}
              >
                <RefreshCw className="mr-2 h-5 w-5" aria-hidden />
                {t("createNew")}
              </Button>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
