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
import { Toast } from "~/components/ui/toast";
import { normalizeLocale, type AppLocale } from "~/i18n/locales";
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

const toastLabelsByLocale = {
  de: {
    close: "Schließen",
    typeTitles: {
      success: "Erfolgreich!",
      error: "Fehler",
      info: "Information",
      warning: "Warnung",
    },
    actionInstruction: (label: string) => `Tippen zum ${label}`,
  },
  en: {
    close: "Close",
    typeTitles: {
      success: "Success!",
      error: "Error",
      info: "Information",
      warning: "Warning",
    },
    actionInstruction: (label: string) => `Tap to ${label}`,
  },
  ru: {
    close: "Закрыть",
    typeTitles: {
      success: "Успешно!",
      error: "Ошибка",
      info: "Информация",
      warning: "Предупреждение",
    },
    actionInstruction: (label: string) => `Нажмите: ${label}`,
  },
  sq: {
    close: "Mbyll",
    typeTitles: {
      success: "Me sukses!",
      error: "Gabim",
      info: "Informacion",
      warning: "Paralajmërim",
    },
    actionInstruction: (label: string) => `Prekni: ${label}`,
  },
  pl: {
    close: "Zamknij",
    typeTitles: {
      success: "Sukces!",
      error: "Błąd",
      info: "Informacja",
      warning: "Ostrzeżenie",
    },
    actionInstruction: (label: string) => `Dotknij: ${label}`,
  },
  tr: {
    close: "Kapat",
    typeTitles: {
      success: "Başarılı!",
      error: "Hata",
      info: "Bilgi",
      warning: "Uyarı",
    },
    actionInstruction: (label: string) => `Dokunun: ${label}`,
  },
  uk: {
    close: "Закрити",
    typeTitles: {
      success: "Успішно!",
      error: "Помилка",
      info: "Інформація",
      warning: "Попередження",
    },
    actionInstruction: (label: string) => `Торкніться: ${label}`,
  },
} as const satisfies Record<
  AppLocale,
  {
    readonly close: string;
    readonly typeTitles: Readonly<Record<ToastType, string>>;
    actionInstruction: (label: string) => string;
  }
>;

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

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

function ToastRow({
  item,
  onClose,
  reducedMotion,
  isMobile,
  locale,
}: Readonly<{
  item: ToastItemData;
  onClose: (id: string) => void;
  reducedMotion: boolean;
  isMobile: boolean;
  locale: AppLocale;
}>) {
  const labels = toastLabelsByLocale[locale];

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
    <Toast
      type={item.type}
      message={item.message}
      accessibleLabel={`${labels.typeTitles[item.type]}: ${item.message}`}
      closeLabel={labels.close}
      onClose={dismissWithExitAnimation}
      action={
        item.action
          ? {
              label: item.action.label,
              accessibleLabel: isMobile
                ? labels.actionInstruction(item.action.label)
                : item.action.label,
              onClick: handleAction,
            }
          : undefined
      }
      visible={visible && !exiting}
      reducedMotion={reducedMotion}
      touchFriendly={isMobile}
      onMouseEnter={pauseIfDesktop}
      onMouseLeave={resumeIfDesktop}
    />
  );
}

export function ToastProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const [items, setItems] = useState<ToastItemData[]>([]);
  const reducedMotion = useReducedMotion();
  const isMobile = useMediaQuery(BELOW_MD);
  const locale = normalizeLocale(
    typeof document === "undefined" ? "de" : document.documentElement.lang,
  );

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

      // Keep short, passive feedback out of the way; callers with an action or
      // a longer explanation opt into a longer duration.
      const duration = options?.duration ?? 1500;

      // A breadcrumb, not a Sentry event: the caller that failed reports the
      // cause itself, so an error here would count every failure twice (#3694).
      if (type === "error") {
        logger.warn("user-facing error displayed", {
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

  return (
    <ToastContext.Provider value={api}>
      {children}

      <div className="pointer-events-none fixed inset-x-4 bottom-[calc(5.5rem+env(safe-area-inset-bottom))] z-[9000] mx-auto flex max-w-sm flex-col gap-2 md:hidden">
        {isMobile &&
          items.map((item) => (
            <ToastRow
              key={item.id}
              item={item}
              onClose={remove}
              reducedMotion={reducedMotion}
              isMobile={isMobile}
              locale={locale}
            />
          ))}
      </div>

      <div className="pointer-events-none fixed right-6 bottom-6 z-[9000] hidden max-w-sm flex-col items-stretch justify-end gap-2 md:flex">
        {!isMobile &&
          items.map((item) => (
            <ToastRow
              key={item.id}
              item={item}
              onClose={remove}
              reducedMotion={reducedMotion}
              isMobile={isMobile}
              locale={locale}
            />
          ))}
      </div>
    </ToastContext.Provider>
  );
}
