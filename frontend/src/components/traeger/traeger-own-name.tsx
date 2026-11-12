"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { createLogger } from "~/lib/logger";
import { fetchTraegerProfile, type TraegerProfile } from "~/lib/traeger-api";

const logger = createLogger({ component: "useTraegerOwnName" });

/**
 * Who is signed in to which Träger: the account and the organization of the
 * session. The name belongs to the membership (#4693), so a change of either
 * drops the name read before.
 */
export type TraegerMembershipKey = `${string}:${string}`;

type Publish = (profile: TraegerProfile) => void;

/**
 * The name of the signed-in person in this Träger-Büro, for the header
 * (#6870). It comes from the own profile, which every Träger role may read;
 * no account list is needed. `undefined` while it loads, after a failed read
 * and while the person has no name: the header then keeps the e-mail
 * address of the session.
 *
 * `publish` hands a profile the page „Konto“ loaded or saved to the header,
 * so a new name shows without signing in again.
 */
export function useTraegerOwnName(membership: TraegerMembershipKey | null): {
  name: string | undefined;
  publish: Publish;
} {
  const [loaded, setLoaded] = useState<{
    membership: TraegerMembershipKey;
    name: string;
  } | null>(null);
  const publishedRevision = useRef(0);

  useEffect(() => {
    if (!membership) return;
    let active = true;
    const revision = publishedRevision.current;
    fetchTraegerProfile()
      .then((profile) => {
        if (active && revision === publishedRevision.current) {
          setLoaded({ membership, name: profile.name.trim() });
        }
      })
      .catch((err: unknown) => {
        // Silent on purpose: the header keeps the e-mail address, and the
        // page „Konto“ reports its own load error.
        logger.warn("traeger_own_name_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      active = false;
    };
  }, [membership]);

  const publish = useCallback<Publish>(
    (profile) => {
      if (!membership) return;
      // A profile of another account (a session changed meanwhile) never
      // lands in this header.
      if (!membership.startsWith(`${profile.accountId}:`)) return;
      // A pending header read must not overwrite a profile from „Konto“.
      publishedRevision.current += 1;
      setLoaded({ membership, name: profile.name.trim() });
    },
    [membership],
  );

  const name =
    loaded && loaded.membership === membership && loaded.name
      ? loaded.name
      : undefined;
  return { name, publish };
}

const PublishContext = createContext<Publish>(() => {});

/** Lets the page „Konto“ update the header (see `useTraegerOwnName`). */
export function TraegerOwnNamePublisher({
  publish,
  children,
}: Readonly<{ publish: Publish; children: React.ReactNode }>) {
  return (
    <PublishContext.Provider value={publish}>
      {children}
    </PublishContext.Provider>
  );
}

export function usePublishTraegerOwnProfile(): Publish {
  return useContext(PublishContext);
}
