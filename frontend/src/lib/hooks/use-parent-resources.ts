"use client";

import { useCallback, useEffect, useLayoutEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { ApiError } from "~/lib/api-error";
import { useSWRAuth, useInvalidateMatching } from "~/lib/swr";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import { parentKeys } from "~/lib/swr/parent-keys";
import {
  listMyChildren,
  getChildFeatures,
  getChildToday,
  type Child,
} from "~/lib/parent-api";

import {
  PARENT_ACCESS_REFUSAL_CODES as REFUSAL_CODES,
  isParentAccessRefusal,
  PARENT_ACCESS_REFUSED_EVENT,
} from "~/lib/parent-access-refusal";
const REFUSAL_SIGNAL = "moto:parent-access-refused";
let tabSource: string | undefined;
const sourceOfTab = () => (tabSource ??= crypto.randomUUID());
export const PARENT_RESOURCE_OPTIONS = {
  keepPreviousData: false,
  revalidateOnFocus: true,
  focusThrottleInterval: 60_000,
  dedupingInterval: 60_000,
  revalidateIfStale: false,
} as const;

/** A refusal withdraws every derived page too, not only the endpoint's entry. */
function useParentRefusal() {
  const discard = useDiscardRefusedEntity();
  const invalidate = useInvalidateMatching();
  const { data: session } = useSession();
  const account =
    session?.user.scope === "parent" ? session.user.id : undefined;
  return useCallback(
    (error: unknown, broadcast = true) => {
      if (!account || !isParentAccessRefusal(error)) return false;
      const refused = discard(error, {
        codes: REFUSAL_CODES,
        matches: parentKeys.isResource,
      });
      if (refused) void invalidate((key) => key === parentKeys.shellChildren());
      if (refused && broadcast && account && error instanceof ApiError) {
        const channel = new BroadcastChannel(REFUSAL_SIGNAL);
        channel.postMessage({
          source: sourceOfTab(),
          account,
          code: error.code,
          status: error.status,
        });
        channel.close();
      }
      return refused;
    },
    [discard, invalidate, account],
  );
}

/** Mounted once in the authenticated shell; the standard scope boundary owns logout. */
export function useParentResourceEvents() {
  const invalidate = useInvalidateMatching();
  const refuse = useParentRefusal();
  const { data: session } = useSession();
  const account =
    session?.user.scope === "parent" ? session.user.id : undefined;
  const { data: children } = useParentChildren(!!account);
  const relationships = children ? parentKeys.features(children) : null;
  const previous = useRef({ account, relationships });
  useLayoutEffect(() => {
    const before = previous.current;
    previous.current = { account, relationships };
    if (
      before.account === account &&
      before.relationships !== null &&
      relationships !== null &&
      before.relationships !== relationships
    ) {
      // A list can reveal a removed relationship without a refused endpoint.
      // Do not revive its old capabilities if that relationship is later added again.
      void invalidate(
        (key) =>
          parentKeys.isResource(key) && key !== parentKeys.shellChildren(),
        { clear: true },
      );
    }
  }, [account, relationships, invalidate]);
  useEffect(() => {
    const refresh = () => {
      void invalidate(parentKeys.isResource);
    };
    const accessRefused = (event: Event) => {
      const detail = (
        event as CustomEvent<{ account?: string; error?: unknown }>
      ).detail;
      if (detail?.account === account) refuse(detail.error);
    };
    const channel = new BroadcastChannel(REFUSAL_SIGNAL);
    channel.onmessage = (event: MessageEvent<unknown>) => {
      const signal = event.data;
      if (!signal || typeof signal !== "object") return;
      const {
        source,
        account: signalAccount,
        code,
        status,
      } = signal as Record<string, unknown>;
      if (source === sourceOfTab() || signalAccount !== account) return;
      if (
        (code === "care.child_not_linked" ||
          code === "care.guardian_permission_denied") &&
        (status === 401 || status === 403 || status === 404)
      ) {
        refuse(new ApiError(code, status, { code }), false);
      }
    };
    window.addEventListener("parent-child-status-refresh", refresh);
    window.addEventListener("parent-threads-refresh", refresh);
    window.addEventListener("parent-news-unread-refresh", refresh);
    window.addEventListener(PARENT_ACCESS_REFUSED_EVENT, accessRefused);
    return () => {
      window.removeEventListener("parent-child-status-refresh", refresh);
      window.removeEventListener("parent-threads-refresh", refresh);
      window.removeEventListener("parent-news-unread-refresh", refresh);
      channel.close();
      window.removeEventListener(PARENT_ACCESS_REFUSED_EVENT, accessRefused);
    };
  }, [invalidate, refuse, account]);
}

export function useParentChildren(enabled = true) {
  const { data: session } = useSession();
  return useSWRAuth(
    enabled && session?.user.scope === "parent"
      ? parentKeys.shellChildren()
      : null,
    listMyChildren,
    PARENT_RESOURCE_OPTIONS,
  );
}

/** Every relationship is probed, including siblings at the same school. */
export function useParentFeatures(children: readonly Child[] | undefined) {
  const refuse = useParentRefusal();
  const { data: session } = useSession();
  return useSWRAuth(
    session?.user.scope === "parent" && children
      ? parentKeys.features(children)
      : null,
    async () => {
      const entries = await Promise.all(
        (children ?? []).map(
          async (child) =>
            [
              child.student_id,
              await getChildFeatures(child.student_id),
            ] as const,
        ),
      );
      return Object.fromEntries(entries);
    },
    {
      ...PARENT_RESOURCE_OPTIONS,
      onError: (error) => {
        refuse(error);
      },
    },
  );
}

export function useParentToday(children: readonly Child[] | undefined) {
  const refuse = useParentRefusal();
  const { data: session } = useSession();
  return useSWRAuth(
    session?.user.scope === "parent" && children
      ? parentKeys.today(children)
      : null,
    async () => {
      const entries = await Promise.all(
        (children ?? []).map(
          async (child) =>
            [child.student_id, await getChildToday(child.student_id)] as const,
        ),
      );
      return Object.fromEntries(entries);
    },
    {
      ...PARENT_RESOURCE_OPTIONS,
      onError: (error) => {
        refuse(error);
      },
    },
  );
}
