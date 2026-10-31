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
  type ChildFeatures,
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
    const refreshChild = () => {
      void invalidate(parentKeys.isChildResource);
    };
    const refreshMessages = () => {
      void invalidate(parentKeys.isMessagesResource);
    };
    const refreshNews = () => {
      void invalidate(parentKeys.isNewsResource);
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
    window.addEventListener("parent-child-status-refresh", refreshChild);
    window.addEventListener("parent-threads-refresh", refreshMessages);
    window.addEventListener("parent-news-unread-refresh", refreshNews);
    window.addEventListener(PARENT_ACCESS_REFUSED_EVENT, accessRefused);
    return () => {
      window.removeEventListener("parent-child-status-refresh", refreshChild);
      window.removeEventListener("parent-threads-refresh", refreshMessages);
      window.removeEventListener("parent-news-unread-refresh", refreshNews);
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

// Partial responses are visible only through SWR's current-error gate (G19).
// Deliberately not an ApiError: discarding a refusal must never restamp these
// partial values into the new data generation as a preserved refusal error.
class ParentFeaturesFailure extends Error {
  constructor(
    readonly partial: Record<string, ChildFeatures>,
    readonly failures: Record<string, unknown>,
  ) {
    super("Parent feature lookups failed");
  }
}
const NO_FEATURE_FAILURES: Readonly<Record<string, unknown>> = {};

/** Every relationship is probed, including siblings at the same school. */
export function useParentFeatures(children: readonly Child[] | undefined) {
  const refuse = useParentRefusal();
  const { data: session } = useSession();
  const resource = useSWRAuth(
    session?.user.scope === "parent" && children
      ? parentKeys.features(children)
      : null,
    async () => {
      const list = children ?? [];
      const responses = await Promise.allSettled(
        list.map((child) => getChildFeatures(child.student_id)),
      );
      const partial: Record<string, ChildFeatures> = {};
      const failures: Record<string, unknown> = {};
      responses.forEach((response, index) => {
        const child = list[index];
        if (!child) return;
        if (response.status === "fulfilled")
          partial[child.student_id] = response.value;
        else failures[child.student_id] = response.reason;
      });
      if (Object.keys(failures).length > 0)
        throw new ParentFeaturesFailure(partial, failures);
      return partial;
    },
    {
      ...PARENT_RESOURCE_OPTIONS,
      onError: (error) => {
        // Only original API errors are refusals; never preserve the batch's data-bearing error.
        const failures =
          error instanceof ParentFeaturesFailure
            ? Object.values(error.failures)
            : [error];
        for (const failure of failures) if (refuse(failure)) break;
      },
      shouldRetryOnError: (error) =>
        !(
          error instanceof ParentFeaturesFailure &&
          Object.values(error.failures).some(
            (failure) =>
              failure instanceof ApiError &&
              (failure.status === 401 || failure.status === 403),
          )
        ),
    },
  );
  const batch =
    resource.error instanceof ParentFeaturesFailure
      ? resource.error
      : undefined;
  return {
    ...resource,
    data: resource.error ? batch?.partial : resource.data,
    error: batch ? Object.values(batch.failures)[0] : resource.error,
    errorsByChild: batch?.failures ?? NO_FEATURE_FAILURES,
  };
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
