"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import { Toast } from "~/components/ui/toast";
import { clientEnv } from "~/env.client";
import { normalizeLocale, type AppLocale } from "~/i18n/locales";
import { ApiError } from "~/lib/api-error";
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
  requestId?: string;
  requestIdLabel?: string;
}

interface ToastItemData {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
  action?: ToastOptions["action"];
  requestId?: string;
  requestIdLabel?: string;
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
    copyRequestId: "Vorgangskennung kopieren",
    copySucceeded: "Kopiert.",
    copyFailed: "Kopieren nicht möglich.",
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
    copyRequestId: "Copy request ID",
    copySucceeded: "Copied.",
    copyFailed: "Could not copy.",
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
    copyRequestId: "Скопировать номер запроса",
    copySucceeded: "Скопировано.",
    copyFailed: "Не удалось скопировать.",
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
    copyRequestId: "Kopjo numrin e kërkesës",
    copySucceeded: "U kopjua.",
    copyFailed: "Nuk u kopjua.",
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
    copyRequestId: "Kopiuj numer żądania",
    copySucceeded: "Skopiowano.",
    copyFailed: "Nie udało się skopiować.",
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
    copyRequestId: "İstek kimliğini kopyala",
    copySucceeded: "Kopyalandı.",
    copyFailed: "Kopyalanamadı.",
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
    copyRequestId: "Копіювати номер запиту",
    copySucceeded: "Скопійовано.",
    copyFailed: "Не вдалося скопіювати.",
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
    readonly copyRequestId: string;
    readonly copySucceeded: string;
    readonly copyFailed: string;
    readonly typeTitles: Readonly<Record<ToastType, string>>;
    actionInstruction: (label: string) => string;
  }
>;

// Kept small so a failed presenter chunk can still show a localized message.
const emergencyErrorByLocale: Record<AppLocale, string> = {
  de: "{object} konnte nicht bearbeitet werden. Bitte versuchen Sie es später.",
  en: "{object} could not be processed. Please try later.",
  pl: "Nie udało się przetworzyć: {object}. Spróbuj później.",
  ru: "Не удалось обработать: {object}. Повторите позже.",
  sq: "Nuk u përpunua: {object}. Provoni më vonë.",
  tr: "{object} işlenemedi. Daha sonra deneyin.",
  uk: "Не вдалося обробити: {object}. Спробуйте пізніше.",
};

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
      requestId={item.requestId}
      requestIdLabel={item.requestIdLabel}
      copyRequestIdLabel={labels.copyRequestId}
      copySucceededLabel={labels.copySucceeded}
      copyFailedLabel={labels.copyFailed}
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

      // Errors must not disappear on a timer; success feedback lasts four seconds.
      const duration =
        type === "error"
          ? 0
          : type === "success"
            ? 4000
            : (options?.duration ?? 1500);

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
          {
            id,
            type,
            message,
            duration,
            action: options?.action,
            requestId: options?.requestId,
            requestIdLabel: options?.requestIdLabel,
          },
        ];
        if (next.length > MAX_VISIBLE) {
          // Never evict an error that the person has not dismissed.
          const removable = next.findIndex((item) => item.type !== "error");
          if (removable >= 0) next.splice(removable, 1);
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

      <div className="pointer-events-none fixed inset-x-4 bottom-[calc(5.5rem+env(safe-area-inset-bottom))] z-[9000] mx-auto flex max-h-[70vh] max-w-sm flex-col gap-2 overflow-y-auto md:hidden">
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

      <div className="pointer-events-none fixed right-6 bottom-6 z-[9000] hidden max-h-[70vh] max-w-sm flex-col items-stretch justify-end gap-2 overflow-y-auto md:flex">
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

interface ApiErrorDisplayOptions {
  /** Localized noun phrase with article, for example "die Gruppe". */
  object: string;
  retry?: () => void;
}

export function loginUrl(host: string, path: string): string {
  if (
    host === clientEnv.NEXT_PUBLIC_PARENTS_HOSTNAME ||
    path.startsWith("/parents/")
  ) {
    return `${host === clientEnv.NEXT_PUBLIC_PARENTS_HOSTNAME ? "" : "/parents"}/login?error=SessionExpired`;
  }
  if (
    host === clientEnv.NEXT_PUBLIC_SCHOOL_HOSTNAME ||
    path.startsWith("/school/")
  ) {
    return `${host === clientEnv.NEXT_PUBLIC_SCHOOL_HOSTNAME ? "" : "/school"}/login?error=SessionExpired`;
  }
  if (
    host === clientEnv.NEXT_PUBLIC_OPERATOR_HOSTNAME ||
    path.startsWith("/operator/")
  ) {
    return `${host === clientEnv.NEXT_PUBLIC_OPERATOR_HOSTNAME ? "" : "/operator"}/login?error=SessionExpired`;
  }
  return "/?error=SessionExpired";
}

/** Opt-in display path for screens migrated in #2520. */
export function useApiErrorDisplay(
  formRef?: RefObject<HTMLFormElement | null>,
) {
  const toast = useToast();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    const firstField = Object.keys(fieldErrors)[0];
    if (!firstField) return;
    const controls = (formRef?.current ?? document).querySelectorAll<
      HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement
    >("input[name], textarea[name], select[name]");
    const control = [...controls].find((item) => item.name === firstField);
    control?.focus();
  }, [fieldErrors, formRef]);

  const show = useCallback(
    async (error: unknown, options: ApiErrorDisplayOptions) => {
      const locale = normalizeLocale(document.documentElement.lang);
      // The provider is mounted on every route. Load the catalog only when a
      // screen actually uses the opt-in error path.
      let presenter: typeof import("~/lib/error-presentation");
      try {
        presenter = await import("~/lib/error-presentation");
      } catch {
        if (error instanceof ApiError && error.status === 401) {
          window.location.assign(
            loginUrl(window.location.host, window.location.pathname),
          );
          return;
        }
        setFieldErrors({});
        const message = emergencyErrorByLocale[locale].replace(
          "{object}",
          options.object,
        );
        toast.error(message.charAt(0).toUpperCase() + message.slice(1));
        return;
      }
      const { presentError, errorDisplayLabels } = presenter;
      const presentation = presentError(error, options.object, locale);
      if (presentation.requiresLogin) {
        window.location.assign(
          loginUrl(window.location.host, window.location.pathname),
        );
        return presentation;
      }
      setFieldErrors(
        Object.fromEntries(
          presentation.fields.map((field) => [
            field,
            errorDisplayLabels(locale).fieldCheck,
          ]),
        ),
      );
      toast.error(presentation.message, {
        action:
          presentation.retryable && options.retry
            ? {
                label: errorDisplayLabels(locale).retry,
                onClick: options.retry,
              }
            : undefined,
        requestId: presentation.requestId,
        requestIdLabel: errorDisplayLabels(locale).requestId,
      });
      return presentation;
    },
    [toast],
  );

  return {
    show,
    fieldError: (name: string) => fieldErrors[name],
    clearFieldErrors: () => setFieldErrors({}),
  };
}
