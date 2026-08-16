"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { useLocale, useTranslations } from "next-intl";
import type { AppLocale } from "~/i18n/locales";
import { normalizeLocale, writeLocaleCookie } from "~/i18n/locales";
import { fetchParentProfile, updateParentPortalLocale } from "~/lib/parent-api";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { reloadForLocaleChange } from "~/lib/parent-locale-navigation";

const logger = createLogger({ component: "ParentLocaleContext" });
const UNSYNCED_LOCALE_STORAGE_KEY = "phoenix_parent_unsynced_locale";

interface ParentLocaleContextValue {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => Promise<void>;
  authenticated: boolean;
}

type LocalChoiceLocale = {
  readonly storageKey: string;
  readonly locale: AppLocale;
};

const ParentLocaleContext = createContext<ParentLocaleContextValue | null>(
  null,
);

function readUnsyncedLocale(storageKey: string): AppLocale | null {
  if (typeof window === "undefined") return null;
  try {
    const value = window.localStorage.getItem(storageKey);
    return value ? normalizeLocale(value) : null;
  } catch (error) {
    logger.warn("parent_locale_unsynced_read_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

function writeUnsyncedLocale(storageKey: string, locale: AppLocale): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey, locale);
  } catch (error) {
    logger.warn("parent_locale_unsynced_write_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
}

function clearUnsyncedLocale(storageKey: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(storageKey);
  } catch (error) {
    logger.warn("parent_locale_unsynced_clear_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
}

export function ParentLocaleProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // Authentication state is derived from the parent session, not passed in.
  // This provider always sits inside ParentProviders' SessionProvider, so it
  // can read the session directly — a single instance covers anonymous pages
  // (login, status === "unauthenticated") and authenticated ones without the
  // tree having to mount a second, shadowing provider. The profile sync effect
  // re-runs when `status` flips to "authenticated".
  const { data: session, status } = useSession();
  const authenticated = status === "authenticated";
  const toast = useToast();
  const t = useTranslations("language");
  // Server-resolved locale: the cookie if present, else Accept-Language. This
  // is also what the tree was already server-rendered in.
  const intlLocale = normalizeLocale(useLocale());
  const [locale, setLocaleState] = useState<AppLocale>(intlLocale);
  const localChoiceLocaleRef = useRef<LocalChoiceLocale | null>(null);
  const localeSaveTailRef = useRef<Promise<void>>(Promise.resolve());
  const localeChoiceSequenceRef = useRef(0);
  const unsyncedLocaleStorageKey = session?.user?.id
    ? `${UNSYNCED_LOCALE_STORAGE_KEY}:${session.user.id}`
    : UNSYNCED_LOCALE_STORAGE_KEY;

  const persistParentLocale = useCallback((nextLocale: AppLocale) => {
    const savePromise = localeSaveTailRef.current.then(() =>
      updateParentPortalLocale(nextLocale),
    );
    localeSaveTailRef.current = savePromise.then(
      () => undefined,
      () => undefined,
    );
    return savePromise;
  }, []);

  useEffect(() => {
    if (!authenticated) return;
    let cancelled = false;
    async function syncFromProfile() {
      try {
        const profile = await fetchParentProfile();
        if (cancelled) return;
        const stored = profile.portal_locale
          ? normalizeLocale(profile.portal_locale)
          : null;
        const localChoice =
          localChoiceLocaleRef.current?.storageKey === unsyncedLocaleStorageKey
            ? localChoiceLocaleRef.current.locale
            : null;
        const effectiveLocalChoice =
          localChoice ?? readUnsyncedLocale(unsyncedLocaleStorageKey);

        if (effectiveLocalChoice) {
          localChoiceLocaleRef.current = {
            storageKey: unsyncedLocaleStorageKey,
            locale: effectiveLocalChoice,
          };
          if (stored !== effectiveLocalChoice) {
            // A local change can be ahead of, or intentionally unsynced from,
            // the profile. Do not let an old profile GET roll it back.
            setLocaleState(effectiveLocalChoice);
            writeLocaleCookie(effectiveLocalChoice);
            if (effectiveLocalChoice !== intlLocale) {
              reloadForLocaleChange();
            }
            return;
          }
          localChoiceLocaleRef.current = null;
          clearUnsyncedLocale(unsyncedLocaleStorageKey);
        }

        if (!stored) {
          // The guardian has never chosen a portal language. Honour the
          // anonymous locale they arrived with (their pre-login switcher
          // choice via the cookie, or the browser's Accept-Language) instead
          // of snapping to German, and persist it so even an explicit German
          // choice is no longer indistinguishable from "never chosen". The
          // tree was already server-rendered in intlLocale, so no refresh is
          // needed.
          setLocaleState(intlLocale);
          try {
            await persistParentLocale(intlLocale);
          } catch (error) {
            logger.warn("parent_profile_language_adopt_failed", {
              error: error instanceof Error ? error.message : String(error),
            });
          }
          return;
        }

        // An explicit portal choice is the source of truth.
        setLocaleState(stored);
        writeLocaleCookie(stored);
        // The page was server-rendered with intlLocale. When the stored
        // preference differs, refresh the server tree so the whole portal
        // re-renders in it after the locale cookie changes.
        if (stored !== intlLocale) {
          reloadForLocaleChange();
        }
      } catch (error: unknown) {
        logger.warn("parent_profile_language_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }
    void syncFromProfile();
    return () => {
      cancelled = true;
    };
  }, [
    authenticated,
    intlLocale,
    persistParentLocale,
    unsyncedLocaleStorageKey,
  ]);

  const setLocale = useCallback(
    async (nextLocale: AppLocale) => {
      const choiceSequence = localeChoiceSequenceRef.current + 1;
      localeChoiceSequenceRef.current = choiceSequence;
      setLocaleState(nextLocale);
      writeLocaleCookie(nextLocale);
      if (authenticated) {
        localChoiceLocaleRef.current = {
          storageKey: unsyncedLocaleStorageKey,
          locale: nextLocale,
        };
        writeUnsyncedLocale(unsyncedLocaleStorageKey, nextLocale);
        try {
          await persistParentLocale(nextLocale);
          if (
            localeChoiceSequenceRef.current === choiceSequence &&
            localChoiceLocaleRef.current?.storageKey ===
              unsyncedLocaleStorageKey &&
            localChoiceLocaleRef.current.locale === nextLocale
          ) {
            clearUnsyncedLocale(unsyncedLocaleStorageKey);
          }
        } catch (error) {
          logger.warn("parent_profile_language_update_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
          if (localeChoiceSequenceRef.current !== choiceSequence) {
            return;
          }
          // The cookie already switched the UI on this device, but the choice
          // never reached the account, so it won't follow the parent to another
          // browser. Tell them rather than letting a failed save look like a
          // success — the backend deliberately refuses to fake persistence, and
          // the UI shouldn't paper over it either.
          toast.error(t("saveError"));
        }
      }
      // A document reload guarantees the server tree reads the new cookie.
      if (localeChoiceSequenceRef.current === choiceSequence) {
        reloadForLocaleChange();
      }
    },
    [authenticated, persistParentLocale, toast, t, unsyncedLocaleStorageKey],
  );

  const value = useMemo(
    () => ({ locale, setLocale, authenticated }),
    [locale, setLocale, authenticated],
  );

  return (
    <ParentLocaleContext.Provider value={value}>
      {children}
    </ParentLocaleContext.Provider>
  );
}

export function useParentLocale(): ParentLocaleContextValue | null {
  return useContext(ParentLocaleContext);
}
