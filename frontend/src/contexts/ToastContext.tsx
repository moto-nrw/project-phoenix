"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { createLogger } from "~/lib/logger";
import { BELOW_MD, useMediaQuery } from "~/lib/hooks/use-media-query";

// Logger instance for toast notifications
const logger = createLogger({ component: "ToastContext" });

type ToastType = "success" | "error" | "info" | "warning";

interface ToastOptions {
  id?: string;
  duration?: number; // ms
  action?: {
    label: string;
    onClick: () => void;
  };
}

interface ToastItemData {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
  action?: ToastOptions["action"];
}

interface ToastAPI {
  success: (message: string, options?: ToastOptions) => void;
  error: (message: string, options?: ToastOptions) => void;
  info: (message: string, options?: ToastOptions) => void;
  warning: (message: string, options?: ToastOptions) => void;
  remove: (id: string) => void;
}

const ToastContext = createContext<ToastAPI | undefined>(undefined);

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

// Mobile: White background with colored icons (center-overlay style)
const mobileStylesByType: Record<
  ToastType,
  {
    bg: string;
    border: string;
    text: string;
    iconColor: string;
    iconPath: string;
  }
> = {
  success: {
    bg: "bg-white/95",
    border: "border-gray-200",
    text: "text-gray-900",
    iconColor: "text-moto-green",
    iconPath: "M5 13l4 4L19 7",
  },
  error: {
    bg: "bg-white/95",
    border: "border-gray-200",
    text: "text-gray-900",
    iconColor: "text-moto-red",
    iconPath: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  info: {
    bg: "bg-white/95",
    border: "border-gray-200",
    text: "text-gray-900",
    iconColor: "text-moto-blue",
    iconPath: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  warning: {
    bg: "bg-white/95",
    border: "border-gray-200",
    text: "text-gray-900",
    iconColor: "text-moto-orange",
    iconPath:
      "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
  },
};

// Desktop: Original transparent background (bottom-right notification style)
const desktopStylesByType: Record<
  ToastType,
  { bg: string; border: string; text: string; iconPath: string }
> = {
  success: {
    bg: "bg-moto-green/10",
    border: "border-moto-green/20",
    text: "text-moto-green-strong",
    iconPath: "M5 13l4 4L19 7",
  },
  error: {
    bg: "bg-moto-red/10",
    border: "border-moto-red/20",
    text: "text-moto-red-strong",
    iconPath: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  info: {
    bg: "bg-moto-blue/10",
    border: "border-moto-blue/20",
    text: "text-[#4070C8]",
    iconPath: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  warning: {
    bg: "bg-moto-orange/10",
    border: "border-moto-orange/20",
    text: "text-[#C56F0D]",
    iconPath:
      "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
  },
};

const modalTitles: Record<ToastType, string> = {
  success: "Erfolgreich!",
  error: "Fehler",
  info: "Information",
  warning: "Warnung",
};

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof globalThis === "undefined" || !globalThis.matchMedia) return;
    const media = globalThis.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener?.("change", update);
    return () => media.removeEventListener?.("change", update);
  }, []);
  return reduced;
}

interface InternalToastTimers {
  timeoutId?: ReturnType<typeof setTimeout>;
  remaining: number;
  start: number;
}

function MobileToastCard({
  item,
  onSelect,
}: Readonly<{
  item: ToastItemData;
  onSelect: (item: ToastItemData) => void;
}>) {
  const styles = mobileStylesByType[item.type];

  return (
    <Button
      type="button"
      variant="surface"
      size="card"
      aria-label={`${modalTitles[item.type]}: ${item.message}. ${
        item.action ? `Tippen zum ${item.action.label}` : "Tippen zum Schließen"
      }`}
      onClick={() => onSelect(item)}
      className={`${styles.bg} ${styles.border} border text-center shadow-none`}
    >
      <output
        aria-live="polite"
        aria-atomic="true"
        className="flex flex-col items-center gap-3"
      >
        <div className={styles.iconColor}>
          <svg
            className="h-12 w-12"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d={styles.iconPath}
            />
          </svg>
        </div>
        <div>
          <p className={`text-lg font-semibold ${styles.text}`}>
            {modalTitles[item.type]}
          </p>
          <p className={`mt-1 text-sm ${styles.text} opacity-80`}>
            {item.message}
          </p>
        </div>
      </output>
    </Button>
  );
}

function ToastRow({
  item,
  onClose,
  reducedMotion,
  isMobile,
}: Readonly<{
  item: ToastItemData;
  onClose: (id: string) => void;
  reducedMotion: boolean;
  isMobile: boolean;
}>) {
  const desktopStyles = desktopStylesByType[item.type];

  const [visible, setVisible] = useState(false);
  const [exiting, setExiting] = useState(false);
  const timersRef = useRef<InternalToastTimers>({
    remaining: item.duration,
    start: Date.now(),
  });

  useEffect(() => {
    setVisible(true);
    timersRef.current.start = Date.now();

    let localTimeout: ReturnType<typeof setTimeout> | undefined;
    if (item.duration > 0) {
      localTimeout = setTimeout(() => {
        setExiting(true);
        setTimeout(() => onClose(item.id), reducedMotion ? 0 : 300);
      }, item.duration);
      timersRef.current.timeoutId = localTimeout;
    }

    return () => {
      if (localTimeout) clearTimeout(localTimeout);
    };
  }, [item.duration, item.id, onClose, reducedMotion]);

  const pauseIfDesktop = () => {
    if (isMobile) return;
    if (timersRef.current.timeoutId) {
      clearTimeout(timersRef.current.timeoutId);
      timersRef.current.timeoutId = undefined;
      const elapsed = Date.now() - timersRef.current.start;
      timersRef.current.remaining = Math.max(0, item.duration - elapsed);
    }
  };

  const resumeIfDesktop = () => {
    if (isMobile) return;
    if (timersRef.current.remaining > 0) {
      timersRef.current.start = Date.now();
      timersRef.current.timeoutId = setTimeout(() => {
        setExiting(true);
        setTimeout(() => onClose(item.id), reducedMotion ? 0 : 300);
      }, timersRef.current.remaining);
    }
  };

  const dismissWithExitAnimation = () => {
    if (timersRef.current.timeoutId) {
      clearTimeout(timersRef.current.timeoutId);
      timersRef.current.timeoutId = undefined;
    }
    setExiting(true);
    setTimeout(() => onClose(item.id), reducedMotion ? 0 : 300);
  };

  const handleAction = () => {
    item.action?.onClick();
    dismissWithExitAnimation();
  };

  return (
    <output
      aria-live="polite"
      aria-atomic="true"
      aria-hidden={isMobile}
      onMouseEnter={pauseIfDesktop}
      onMouseLeave={resumeIfDesktop}
      className={`pointer-events-auto hidden md:block ${desktopStyles.bg} ${desktopStyles.border} ${desktopStyles.text} rounded-2xl border p-4 shadow-lg backdrop-blur-sm transition-all ${reducedMotion ? "" : "duration-300 ease-out"} ${visible && !exiting ? "translate-y-0 opacity-100" : "translate-y-2 opacity-0"}`}
    >
      <div className="flex items-start gap-3">
        <div className={`flex-shrink-0 ${desktopStyles.text}`}>
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d={desktopStyles.iconPath}
            />
          </svg>
        </div>
        <p className={`flex-1 text-sm font-medium ${desktopStyles.text}`}>
          {item.message}
        </p>
        {item.action && (
          <button
            type="button"
            onClick={handleAction}
            className={`flex-shrink-0 self-center text-sm font-semibold ${desktopStyles.text} underline underline-offset-2 transition-opacity hover:opacity-70`}
          >
            {item.action.label}
          </button>
        )}
        <button
          type="button"
          aria-label="Schließen"
          onClick={() => onClose(item.id)}
          className={`flex-shrink-0 ${desktopStyles.text} transition-opacity hover:opacity-70`}
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </output>
  );
}

export function ToastProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const [items, setItems] = useState<ToastItemData[]>([]);
  const reducedMotion = useReducedMotion();
  const isMobile = useMediaQuery(BELOW_MD);

  // Track last shown timestamps for simple de-duplication
  const lastShownRef = useRef<Map<string, number>>(new Map());
  const DE_DUPE_WINDOW = 2000; // ms
  const MAX_VISIBLE = 3;

  const remove = useCallback((id: string) => {
    setItems((prev) => prev.filter((it) => it.id !== id));
  }, []);

  const push = useCallback(
    (type: ToastType, message: string, options?: ToastOptions) => {
      if (!message) return;
      const now = Date.now();
      const last = lastShownRef.current.get(message) ?? 0;
      if (now - last < DE_DUPE_WINDOW) return; // de-dup
      lastShownRef.current.set(message, now);

      const id =
        options?.id ?? `${now}-${Math.random().toString(36).slice(2, 8)}`;

      // Use shorter duration for mobile center-overlay style
      // Default to 1500ms to match mobile UX, desktop can be longer if needed
      const duration = options?.duration ?? 1500;

      // Log error toasts for monitoring
      if (type === "error") {
        logger.error("user-facing error displayed", {
          message: message.substring(0, 100), // Truncate for logging
          toast_type: type,
          source: "toast_context",
        });
      }

      setItems((prev) => {
        const next: ToastItemData[] = [
          ...prev,
          { id, type, message, duration, action: options?.action },
        ];
        if (next.length > MAX_VISIBLE) {
          // remove oldest to keep at most MAX_VISIBLE visible
          next.shift();
        }
        return next;
      });
    },
    [],
  );

  const api: ToastAPI = useMemo(
    () => ({
      success: (m, o) => push("success", m, o),
      error: (m, o) => push("error", m, o),
      info: (m, o) => push("info", m, o),
      warning: (m, o) => push("warning", m, o),
      remove,
    }),
    [push, remove],
  );

  const topmostItem = items.at(-1);
  const handleMobileSelect = useCallback(
    (item: ToastItemData) => {
      item.action?.onClick();
      remove(item.id);
    },
    [remove],
  );

  return (
    <ToastContext.Provider value={api}>
      {children}

      <Modal
        isOpen={isMobile && topmostItem !== undefined}
        onClose={() => {
          if (topmostItem) remove(topmostItem.id);
        }}
        title="Benachrichtigungen"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-xs"
        backdropLabel="Benachrichtigung schließen"
      >
        <div className="space-y-2">
          {items.map((item) => (
            <MobileToastCard
              key={item.id}
              item={item}
              onSelect={handleMobileSelect}
            />
          ))}
        </div>
      </Modal>

      <div className="pointer-events-none fixed right-6 bottom-6 z-[9000] hidden max-w-sm flex-col items-stretch justify-end gap-2 md:flex">
        {items.map((item) => (
          <ToastRow
            key={item.id}
            item={item}
            onClose={remove}
            reducedMotion={reducedMotion}
            isMobile={isMobile}
          />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
