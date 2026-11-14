"use client";

import { useEffect, useRef, useState } from "react";
import { ButtonLink } from "~/components/ui/button";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { DEMO_BOOKING_URL } from "~/lib/demo-build";

// HubSpot's scheduler has a fixed layout height and reports it in `height`
// messages. After a booking it reports the removed form (25 px); the last
// real height stays.
const DEFAULT_FRAME_HEIGHT = 715;
const MIN_FRAME_HEIGHT = 300;
const READY_FALLBACK_MS = 1500;
// Below this width HubSpot switches to its own scrolling phone layout.
const SCALE_MIN_WIDTH = 640;
// Below this scale the text and touch targets get too small; the calendar
// keeps its size and scrolls inside the frame instead.
const SCALE_MIN = 0.6;

const BOOKING_ORIGIN = new URL(DEMO_BOOKING_URL).origin;
const EMBED_URL = (() => {
  const url = new URL(DEMO_BOOKING_URL);
  url.searchParams.set("embed", "true");
  return url.toString();
})();

/** How far the calendar shrinks to fit a box of this size; 1 keeps it. */
export function calendarScale(
  width: number,
  height: number,
  frameHeight: number,
): number {
  if (width < SCALE_MIN_WIDTH) return 1;
  const scale = height / frameHeight;
  // A scale of about 0.99 would only blur the text.
  if (scale >= 0.98 || scale < SCALE_MIN) return 1;
  return Math.round(scale * 1000) / 1000;
}

interface DemoBookingDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

/**
 * The website's booking calendar inside the demo, so a visitor books without
 * leaving it. Like the website, the embed asks HubSpot not to track.
 */
export function DemoBookingDialog({ isOpen, onClose }: DemoBookingDialogProps) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Termin buchen"
      titleLevel={2}
      description={
        <>
          Wählen Sie einen freien Termin mit dem moto-Team.{" "}
          <ButtonLink
            href={DEMO_BOOKING_URL}
            target="_blank"
            rel="noopener noreferrer"
            variant="link"
          >
            In neuem Tab öffnen
          </ButtonLink>
        </>
      }
      size="xl"
      mobileSheet
    >
      {isOpen ? <BookingFrame /> : null}
    </Modal>
  );
}

function BookingFrame() {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const boxRef = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);
  const [frameHeight, setFrameHeight] = useState(DEFAULT_FRAME_HEIGHT);
  const [scale, setScale] = useState(1);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      const frameWindow = frameRef.current?.contentWindow;
      if (
        !frameWindow ||
        event.source !== frameWindow ||
        event.origin !== BOOKING_ORIGIN
      ) {
        return;
      }
      if (event.data === "readyForConsentListener") {
        frameWindow.postMessage(
          { type: "privacy-consent", shouldTrackAnalytics: false },
          BOOKING_ORIGIN,
        );
        return;
      }
      const data: unknown = event.data;
      const message =
        typeof data === "object" && data !== null
          ? (data as { height?: unknown; type?: unknown })
          : {};
      const height = Number(message.height);
      if (height >= MIN_FRAME_HEIGHT) setFrameHeight(height);
      if (height > 0 || message.type === "iframe-title") setReady(true);
    };
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, []);

  // HubSpot's desktop layout does not shrink. On a short screen the whole
  // calendar is scaled down instead of scrolling inside the dialog, as on
  // the website.
  useEffect(() => {
    const box = boxRef.current;
    if (!box || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) return;
      const { width, height } = entry.contentRect;
      setScale(calendarScale(width, height, frameHeight));
    });
    observer.observe(box);
    return () => observer.disconnect();
  }, [frameHeight]);

  return (
    <div
      ref={boxRef}
      className="relative overflow-hidden"
      style={{ height: `min(${frameHeight}px, calc(100dvh - 12rem))` }}
    >
      {ready ? null : (
        <div className="absolute inset-0">
          <Loading message="Der Kalender lädt …" />
        </div>
      )}
      <iframe
        ref={frameRef}
        src={EMBED_URL}
        title="Kalender für einen Termin mit dem moto-Team"
        referrerPolicy="no-referrer"
        // HubSpot needs its scripts, the form and new-tab links (video call,
        // calendar entry). The sandbox still blocks top navigation.
        // oxlint-disable-next-line react/iframe-missing-sandbox -- the frame is HubSpot's own origin, never ours: allow-same-origin keeps its origin for its API and the consent message, it grants no access to this page
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox"
        className={`border-0 ${
          scale < 1 ? "absolute top-0 left-0 origin-top-left" : "h-full w-full"
        } ${ready ? "opacity-100" : "opacity-0"}`}
        style={
          scale < 1
            ? {
                width: `${100 / scale}%`,
                height: frameHeight,
                transform: `scale(${scale})`,
              }
            : undefined
        }
        onLoad={() =>
          window.setTimeout(() => setReady(true), READY_FALLBACK_MS)
        }
      />
    </div>
  );
}
