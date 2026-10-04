"use client";

import { useEffect, useState } from "react";
import { Check, Copy, RefreshCw, Rss } from "lucide-react";

import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import {
  createRequestFeed,
  getRequestFeedStatus,
  rotateRequestFeed,
} from "~/lib/request-feed-api";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";

/** Objekt der Fehlertexte aus dem Katalog (#2513). */
const FEED_OBJECT = "das Abo";

interface RequestFeedDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

export function RequestFeedDialog({ isOpen, onClose }: RequestFeedDialogProps) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const { copied, copy } = useClipboardCopy("RequestFeedDialog", 2000);
  const [active, setActive] = useState(false);
  const [url, setURL] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [saving, setSaving] = useState(false);
  const [confirmRotate, setConfirmRotate] = useState(false);
  // Zählt die Ladeversuche; „Wiederholen“ erhöht ihn und lädt neu.
  const [loadAttempt, setLoadAttempt] = useState(0);
  const loadFailed = loadError !== null;

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setActive(false);
    setURL(null);
    setConfirmRotate(false);
    clearLoadError();
    setChecking(true);
    void getRequestFeedStatus()
      .then((status) => {
        if (!cancelled) setActive(status.active);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        void showLoadError(err, {
          object: FEED_OBJECT,
          retry: () => setLoadAttempt((attempt) => attempt + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setChecking(false);
      });
    return () => {
      cancelled = true;
    };
  }, [clearLoadError, isOpen, loadAttempt, showLoadError]);

  const create = async () => {
    setSaving(true);
    try {
      const result = await createRequestFeed();
      setURL(result.url);
      setActive(true);
    } catch (err) {
      await showActionError(err, {
        object: FEED_OBJECT,
        retry: () => void create(),
      });
    } finally {
      setSaving(false);
    }
  };

  const rotate = async () => {
    setSaving(true);
    try {
      const result = await rotateRequestFeed();
      setURL(result.url);
      setActive(true);
      setConfirmRotate(false);
      toast.success("Der neue RSS-Link ist bereit.");
    } catch (err) {
      await showActionError(err, {
        object: FEED_OBJECT,
        retry: () => void rotate(),
      });
    } finally {
      setSaving(false);
    }
  };

  const copyURL = async () => {
    if (!url) return;
    if (await copy(url)) {
      toast.success("Der RSS-Link ist kopiert.");
    } else {
      // Kein API-Fehler: die Zwischenablage hat abgelehnt. Der Link steht
      // markierbar im Feld, also nennt der Text den Weg von Hand.
      toast.error(
        "Kopieren hat nicht geklappt. Bitte markieren Sie den Link und kopieren Sie ihn selbst.",
      );
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Neue Anfragen abonnieren"
      mobileSheet
      isDismissDisabled={saving}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Schließen
          </Button>
          {!loadFailed && !active ? (
            <Button
              type="button"
              size="md"
              isLoading={saving}
              disabled={checking}
              loadingText="Link wird erstellt…"
              onClick={() => void create()}
            >
              <Rss className="mr-2 size-4" aria-hidden />
              RSS-Link erstellen
            </Button>
          ) : null}
          {active && !confirmRotate ? (
            <Button
              type="button"
              size="md"
              variant="outline"
              disabled={saving}
              onClick={() => setConfirmRotate(true)}
            >
              <RefreshCw className="mr-2 size-4" aria-hidden />
              Neuen Link erstellen
            </Button>
          ) : null}
          {active && confirmRotate ? (
            <Button
              type="button"
              size="md"
              isLoading={saving}
              loadingText="Link wird erstellt…"
              onClick={() => void rotate()}
            >
              Alten Link ersetzen
            </Button>
          ) : null}
        </div>
      }
    >
      <div className="space-y-4 text-sm leading-6 text-gray-700">
        <p>
          Ein RSS-Programm zeigt Ihnen neue Elternanfragen. Entscheiden können
          Sie nur in moto.
        </p>

        <div className="rounded-xl bg-gray-50 p-4">
          <p className="font-semibold text-gray-800">Gut zu wissen</p>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-gray-600">
            <li>Das Programm prüft selbst. Hinweise können später ankommen.</li>
            <li>Der Link zeigt Anfragen aus den letzten 30 Tagen.</li>
            <li>Namen und andere persönliche Daten stehen nicht im Feed.</li>
            <li>Ein Online-Dienst verarbeitet den Feed außerhalb von moto.</li>
          </ul>
        </div>

        {checking ? <p role="status">RSS-Link wird geprüft…</p> : null}

        <LoadErrorAlert error={loadError} />

        {active && !url ? <p>Es gibt schon einen RSS-Link.</p> : null}

        {confirmRotate ? (
          <div
            role="alert"
            className="border-moto-orange/40 bg-moto-orange-soft rounded-xl border p-4 text-gray-800"
          >
            Der bisherige Link funktioniert danach nicht mehr. Bereits geladene
            Hinweise bleiben im RSS-Programm gespeichert.
          </div>
        ) : null}

        {url ? (
          <div className="space-y-2">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
              <div className="min-w-0 flex-1">
                <Input
                  id="parent-request-feed-url"
                  label="Ihr RSS-Link"
                  readOnly
                  value={url}
                  controlSize="compact"
                  onFocus={(event) => event.target.select()}
                  className="bg-gray-50 text-gray-700"
                />
              </div>
              <Button
                type="button"
                size="md"
                variant={copied ? "success" : "outline"}
                className="min-w-32 shrink-0"
                onClick={() => void copyURL()}
              >
                {copied ? (
                  <Check className="mr-2 size-4" aria-hidden />
                ) : (
                  <Copy className="mr-2 size-4" aria-hidden />
                )}
                {copied ? "Kopiert" : "Kopieren"}
              </Button>
            </div>
            <p className="text-xs leading-5 text-gray-500">
              Fügen Sie diesen Link in Ihrem RSS-Programm ein. Verwalten Sie den
              Link nur hier in moto.
            </p>
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
