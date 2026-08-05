"use client";

import { useState } from "react";
import { CalendarCheck, Copy, Link2, RefreshCw } from "lucide-react";

import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  getParentCalendarFeed,
  rotateParentCalendarFeed,
  type CalendarFeedInfo,
} from "~/lib/personal-calendar-api";

function messageFrom(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

/**
 * Parents-portal panel to subscribe an external calendar (Apple/Google/Outlook)
 * to the family Termine. Once subscribed, new, changed and cancelled
 * appointments sync automatically — no per-event import needed.
 */
export function CalendarSubscribePanel() {
  const toast = useToast();
  const [feed, setFeed] = useState<CalendarFeedInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setFeed(await getParentCalendarFeed());
      setOpen(true);
    } catch (error) {
      toast.error(messageFrom(error, "Abo-Link konnte nicht geladen werden."));
    } finally {
      setLoading(false);
    }
  };

  const rotate = async () => {
    setLoading(true);
    try {
      setFeed(await rotateParentCalendarFeed());
      toast.success("Neuer Abo-Link erstellt. Der alte Link ist ungültig.");
    } catch (error) {
      toast.error(messageFrom(error, "Abo-Link konnte nicht erneuert werden."));
    } finally {
      setLoading(false);
    }
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success("Link kopiert.");
    } catch {
      toast.error("Kopieren nicht möglich. Bitte den Link manuell markieren.");
    }
  };

  return (
    <div className="moto-content-surface rounded-2xl border border-gray-200 p-4 shadow-sm sm:p-6">
      <div className="flex items-start gap-3">
        <div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-[#ECF7DA] text-[#5a8f1e]">
          <CalendarCheck className="h-5 w-5" aria-hidden />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-semibold text-gray-900">
            Kalender abonnieren
          </h2>
          <p className="mt-1 text-sm leading-6 text-gray-600">
            Abonnieren Sie die Termine einmalig in Ihrem Handy-Kalender. Neue,
            geänderte und abgesagte Termine erscheinen danach automatisch – ohne
            jeden Termin einzeln hinzuzufügen.
          </p>

          {!open ? (
            <Button
              type="button"
              size="md"
              className="mt-4"
              isLoading={loading}
              onClick={load}
            >
              <Link2 className="h-4 w-4" aria-hidden />
              Abo-Link anzeigen
            </Button>
          ) : null}

          {open && feed && feed.url ? (
            <div className="mt-4 space-y-4">
              <a
                href={feed.webcal_url}
                className="bg-moto-green inline-flex h-10 items-center justify-center gap-1.5 rounded-md px-4 text-sm font-semibold text-gray-950 transition-colors hover:bg-[#74b827]"
              >
                <CalendarCheck className="h-4 w-4" aria-hidden />
                Im Kalender abonnieren
              </a>

              <div>
                <label
                  htmlFor="calendar-feed-url"
                  className="mb-1 block text-xs font-medium text-gray-500"
                >
                  Abo-Link (zum Kopieren)
                </label>
                <div className="flex items-center gap-2">
                  <input
                    id="calendar-feed-url"
                    readOnly
                    value={feed.url}
                    onFocus={(event) => event.target.select()}
                    className="h-9 w-full rounded-md border-0 bg-gray-50 px-3 text-xs text-gray-700 ring-1 ring-gray-200 ring-inset"
                  />
                  <Button
                    type="button"
                    size="compact"
                    variant="outline"
                    onClick={() => copy(feed.url)}
                  >
                    <Copy className="h-4 w-4" aria-hidden />
                    Kopieren
                  </Button>
                </div>
                <p className="mt-1 text-xs text-gray-500">
                  Aus Sicherheitsgründen wird dieser Link nur jetzt angezeigt.
                  Kopieren Sie ihn gleich in Ihren Kalender.
                </p>
              </div>

              <div className="rounded-lg bg-gray-50 p-3 text-xs leading-5 text-gray-600">
                <p className="font-semibold text-gray-700">So geht&apos;s:</p>
                <p className="mt-1">
                  <span className="font-medium">iPhone/iPad:</span>{" "}
                  Einstellungen → Kalender → Accounts → Account hinzufügen →
                  Andere → Kalenderabo hinzufügen, dann den Link einfügen.
                </p>
                <p className="mt-1">
                  <span className="font-medium">Android:</span> In Google
                  Kalender (am Computer) → Weitere Kalender → Per URL, dann den
                  Link einfügen. Der Kalender synchronisiert dann auch am Handy.
                </p>
              </div>

              <Button
                type="button"
                size="compact"
                variant="ghost"
                className="text-gray-500"
                isLoading={loading}
                onClick={rotate}
              >
                <RefreshCw className="h-4 w-4" aria-hidden />
                Link neu generieren
              </Button>
            </div>
          ) : null}

          {open && feed && !feed.url ? (
            <div className="mt-4 space-y-3">
              <div className="rounded-lg bg-gray-50 p-3 text-sm leading-6 text-gray-600">
                Ihr Abo-Link ist bereits aktiv. Aus Sicherheitsgründen wird er
                nicht erneut angezeigt. Falls Sie ihn nicht mehr haben, erzeugen
                Sie einen neuen Link – der bisherige wird dann ungültig.
              </div>
              <Button
                type="button"
                size="md"
                isLoading={loading}
                onClick={rotate}
              >
                <RefreshCw className="h-4 w-4" aria-hidden />
                Neuen Abo-Link erzeugen
              </Button>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
