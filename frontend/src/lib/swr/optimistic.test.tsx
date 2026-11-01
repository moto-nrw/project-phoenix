import { act, render, screen } from "@testing-library/react";
import type { Session } from "next-auth";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The optimistic helper on the real SWR 2.5.1 and the real scope boundary.
vi.unmock("swr");

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));

import { ApiError } from "~/lib/api-error";
import { _resetPersonalStorageLeave } from "~/lib/personal-storage";
import { useSWRAuth } from "./hooks";
import {
  useOptimisticAction,
  useMessageSendAttempt,
  type OptimisticOutcome,
} from "./optimistic";
import { resetSWRScope, resumeSWRSession } from "./scope";
import { SWRScopeBoundary } from "./scope-boundary";
import { SWRConfig } from "./testing";
import type { SWRResponse } from "swr";

/**
 * Invariants of useOptimisticAction (#5011):
 * - O1 an overlay shows only in the scope and data generation it began in;
 * - O2 after a scope change during the request neither the overlay, nor
 *   the answer, nor a rollback reaches the new scope;
 * - O3 a failure returns exactly to the state before; concurrent actions
 *   keep each other's overlays and commits;
 * - a refusal (401/403 with its code) drops the resource, a business 403
 *   only rolls back.
 */

interface Item {
  readonly id: string;
  readonly body: string;
}
interface Thread {
  readonly items: readonly Item[];
}

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (error: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function signedIn(user: Partial<Session["user"]> = {}) {
  return {
    data: { user: { id: "A", tenantId: 1, ...user }, expires: "2099-01-01" },
    status: "authenticated" as const,
  };
}

async function flush() {
  await act(async () => {
    for (let i = 0; i < 5; i += 1) await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

let reader: SWRResponse<Thread>;
let action: ReturnType<typeof useOptimisticAction<Thread>>;
let attempt: ReturnType<typeof useMessageSendAttempt>;
/** Every list the chat rendered, as `body` or `body:state`. */
let frames: string[] = [];

function label(thread: Thread | undefined): string {
  if (!thread) return "leer";
  return thread.items
    .map((item) => {
      const state = action.stateOf(item.id);
      return state ? `${item.body}:${state}` : item.body;
    })
    .join("|");
}

function Chat({
  fetcher,
  name = "thread-1",
}: Readonly<{ fetcher: () => Promise<Thread>; name?: string }>) {
  reader = useSWRAuth<Thread>(name, fetcher, {
    revalidateOnFocus: false,
  });
  action = useOptimisticAction(name, reader.data);
  attempt = useMessageSendAttempt(name);
  const shown = label(action.data);
  frames.push(shown);
  return <p data-testid="chat">{shown}</p>;
}

/** Another view of the same resource and an unrelated one. */
function Other({
  name,
  fetcher,
}: Readonly<{ name: string; fetcher: () => Promise<string> }>) {
  const { data } = useSWRAuth<string>(name, fetcher);
  return <p data-testid={name}>{data ?? "leer"}</p>;
}

function Page({ children }: Readonly<{ children?: ReactNode }>) {
  return (
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
        shouldRetryOnError: false,
      }}
    >
      <SWRScopeBoundary expectedTenantId={1} />
      {children}
    </SWRConfig>
  );
}

const committed: Item[] = [];

/** Sends `body`; the server answers with the saved item. */
function send(
  body: string,
  request: () => Promise<Item>,
  options: { keepFailed?: boolean; refusal?: boolean } = {},
): Promise<OptimisticOutcome<Item>> {
  // The view's commit: the reader's bound mutate of this render.
  const mutate = reader.mutate;
  return action.run({
    optimistic: (thread, { id }) => ({
      items: [...thread.items, { id, body }],
    }),
    request,
    commit: (saved) => {
      committed.push(saved);
      void mutate(
        (thread) => (thread ? { items: [...thread.items, saved] } : thread),
        { revalidate: false },
      );
    },
    keepFailed: options.keepFailed,
    refusal: options.refusal
      ? {
          codes: ["general.permission"],
          matches: (key) => key === "thread-1" || key === "inbox-1",
        }
      : undefined,
  });
}

const START: Thread = { items: [{ id: "1", body: "a" }] };

async function mounted(
  fetcher: () => Promise<Thread> = () => Promise.resolve(START),
  extra?: ReactNode,
) {
  const view = render(
    <Page>
      <Chat fetcher={fetcher} />
      {extra}
    </Page>,
  );
  await flush();
  // Anker: der bestätigte Stand ist geladen.
  expect(screen.getByTestId("chat")).toHaveTextContent("a");
  frames = [];
  return view;
}

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  mockUseSession.mockReturnValue(signedIn());
  frames = [];
  committed.length = 0;
});

afterEach(() => {
  resumeSWRSession();
  _resetPersonalStorageLeave();
});

describe("useOptimisticAction (#5011)", () => {
  it("shows the overlay in the same task and the answer without a gap or a double", async () => {
    await mounted();
    const answer = deferred<Item>();
    let outcome: Promise<OptimisticOutcome<Item>> = Promise.resolve({
      status: "discarded",
    });
    act(() => {
      outcome = send("b", () => answer.promise);
    });
    // No await: the overlay is on screen before the request answers.
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b:pending");

    await act(async () => {
      answer.resolve({ id: "2", body: "b" });
      await outcome;
    });
    await flush();
    expect(await outcome).toEqual({
      status: "done",
      result: { id: "2", body: "b" },
    });
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b");
    expect(reader.data).toEqual({
      items: [
        { id: "1", body: "a" },
        { id: "2", body: "b" },
      ],
    });
    // Every frame after the start shows the new item exactly once.
    expect(frames.every((frame) => frame.split("|").length === 2)).toBe(true);
  });

  it("returns exactly to the state before on a failure", async () => {
    await mounted();
    const before = reader.data;
    const answer = deferred<Item>();
    let outcome: Promise<OptimisticOutcome<Item>> = Promise.resolve({
      status: "discarded",
    });
    act(() => {
      outcome = send("b", () => answer.promise);
    });
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b:pending");
    const failure = new ApiError("Fehlgeschlagen", 500, {
      code: "general.server",
    });
    await act(async () => {
      answer.reject(failure);
      await outcome;
    });
    expect(await outcome).toEqual({
      status: "failed",
      error: failure,
      refused: false,
    });
    expect(screen.getByTestId("chat")).toHaveTextContent(/^a$/);
    expect(action.data).toBe(before);
    expect(reader.data).toBe(before);
    expect(committed).toEqual([]);
  });

  it("keeps a failed overlay marked until dismiss, without touching the data", async () => {
    await mounted();
    const before = reader.data;
    await act(async () => {
      await send("b", () => Promise.reject(new Error("offline")), {
        keepFailed: true,
      });
    });
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b:failed");
    expect(reader.data).toBe(before);
    act(() => action.dismiss());
    expect(screen.getByTestId("chat")).toHaveTextContent(/^a$/);
    expect(action.data).toBe(before);
  });

  it.each(["unmount", "key change"])(
    "discards a late failure after %s, even when the conversation reopens",
    async (navigation) => {
      const fetcher = () => Promise.resolve(START);
      const view = await mounted(fetcher);
      const answer = deferred<Item>();
      let outcome: Promise<OptimisticOutcome<Item>> = Promise.resolve({
        status: "discarded",
      });
      act(() => {
        outcome = send("b", () => answer.promise, { keepFailed: true });
      });
      expect(screen.getByTestId("chat")).toHaveTextContent("a|b:pending");
      view.rerender(
        <Page>
          {navigation === "key change" ? (
            <Chat name="thread-2" fetcher={fetcher} />
          ) : (
            <p>Andere Seite</p>
          )}
        </Page>,
      );
      // Return before the old request settles: this is a new view lifetime.
      view.rerender(
        <Page>
          <Chat fetcher={fetcher} />
        </Page>,
      );
      await flush();
      await act(async () => {
        answer.reject(new Error("offline"));
        await outcome;
      });
      expect(await outcome).toEqual({ status: "discarded" });
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a$/);
      act(() => action.dismiss());
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a$/);
      expect(committed).toEqual([]);
    },
  );

  it("still commits a successful answer after the sending view unmounts", async () => {
    const fetcher = () => Promise.resolve(START);
    const view = await mounted(fetcher);
    const answer = deferred<Item>();
    let outcome: Promise<OptimisticOutcome<Item>> = Promise.resolve({
      status: "discarded",
    });
    act(() => {
      outcome = send("b", () => answer.promise, { keepFailed: true });
    });
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b:pending");
    view.rerender(
      <Page>
        <p>Andere Seite</p>
      </Page>,
    );
    const saved = { id: "2", body: "b" };
    await act(async () => {
      answer.resolve(saved);
      await outcome;
    });
    expect(await outcome).toEqual({ status: "done", result: saved });
    expect(committed).toEqual([saved]);
    view.rerender(
      <Page>
        <Chat fetcher={() => new Promise<Thread>(() => {})} />
      </Page>,
    );
    await flush();
    expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|b$/);
    expect(reader.data?.items).toEqual([START.items[0], saved]);
  });

  describe("concurrent actions", () => {
    it("drops only the failed one while the other is pending, then keeps its answer", async () => {
      await mounted();
      const first = deferred<Item>();
      const second = deferred<Item>();
      let firstOutcome: Promise<unknown> = Promise.resolve();
      let secondOutcome: Promise<unknown> = Promise.resolve();
      act(() => {
        firstOutcome = send("x", () => first.promise);
        secondOutcome = send("y", () => second.promise);
      });
      expect(screen.getByTestId("chat")).toHaveTextContent(
        "a|x:pending|y:pending",
      );
      await act(async () => {
        first.reject(new Error("x scheiterte"));
        await firstOutcome;
      });
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|y:pending$/);
      await act(async () => {
        second.resolve({ id: "3", body: "y" });
        await secondOutcome;
      });
      await flush();
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|y$/);
    });

    it("keeps an earlier commit when a later action fails", async () => {
      await mounted();
      const first = deferred<Item>();
      const second = deferred<Item>();
      let firstOutcome: Promise<unknown> = Promise.resolve();
      let secondOutcome: Promise<unknown> = Promise.resolve();
      act(() => {
        firstOutcome = send("x", () => first.promise);
        secondOutcome = send("y", () => second.promise);
      });
      await act(async () => {
        first.resolve({ id: "2", body: "x" });
        await firstOutcome;
      });
      await flush();
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|x\|y:pending$/);
      await act(async () => {
        second.reject(new Error("y scheiterte"));
        await secondOutcome;
      });
      await flush();
      expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|x$/);
      expect(reader.data?.items.map((item) => item.body)).toEqual(["a", "x"]);
    });
  });

  describe.each<[string, (rerender: () => void) => void]>([
    [
      "rights change",
      (rerender) => {
        mockUseSession.mockReturnValue(signedIn({ permissions: ["x"] }));
        rerender();
      },
    ],
    [
      "another account in another tab",
      () => {
        globalThis.dispatchEvent(
          new StorageEvent("storage", {
            key: "moto:personal-storage-owner",
            newValue: "1:B",
          }),
        );
      },
    ],
    [
      "logout",
      (rerender) => {
        mockUseSession.mockReturnValue({
          data: null,
          status: "unauthenticated",
        });
        rerender();
      },
    ],
  ])("scope change during the request: %s", (_name, operation) => {
    // Returns the outcome wrapped: an async function would otherwise wait
    // for the request it started.
    async function startThenChange(answer: Promise<Item>) {
      let calls = 0;
      // The new scope loads its own state; the old one never reaches it.
      const fetcher = () => {
        calls += 1;
        return Promise.resolve(
          calls === 1 ? START : { items: [{ id: "9", body: "neu" }] },
        );
      };
      const view = await mounted(fetcher);
      let outcome: Promise<OptimisticOutcome<Item>> = Promise.resolve({
        status: "discarded",
      });
      act(() => {
        outcome = send("b", () => answer, { keepFailed: true });
      });
      expect(screen.getByTestId("chat")).toHaveTextContent("a|b:pending");
      act(() =>
        operation(() =>
          view.rerender(
            <Page>
              <Chat fetcher={fetcher} />
            </Page>,
          ),
        ),
      );
      // The overlay is gone with the scope, before the answer. Frames count
      // from the committed boundary on: the render before its layout effect
      // never reaches the screen (as for every reader, generation-lock).
      expect(screen.getByTestId("chat")).not.toHaveTextContent("b");
      frames = [];
      return { outcome };
    }

    it("shows and commits no answer in the new scope", async () => {
      const answer = deferred<Item>();
      const { outcome } = await startThenChange(answer.promise);
      let result: OptimisticOutcome<Item> = {
        status: "done",
        result: START.items[0]!,
      };
      await act(async () => {
        answer.resolve({ id: "2", body: "b" });
        result = await outcome;
      });
      await flush();
      expect(result).toEqual({ status: "discarded" });
      expect(committed).toEqual([]);
      expect(frames.some((frame) => frame.includes("b"))).toBe(false);
      expect(
        reader.data?.items.some((item) => item.body === "b") ?? false,
      ).toBe(false);
    });

    it("shows no failure and writes no rollback in the new scope", async () => {
      const answer = deferred<Item>();
      const { outcome } = await startThenChange(answer.promise);
      let result: OptimisticOutcome<Item> = {
        status: "done",
        result: START.items[0]!,
      };
      await act(async () => {
        answer.reject(new Error("zu spät"));
        result = await outcome;
      });
      await flush();
      expect(result).toEqual({ status: "discarded" });
      expect(frames.some((frame) => frame.includes("b"))).toBe(false);
      expect(frames.some((frame) => frame.includes("failed"))).toBe(false);
      // The state before the action belongs to the old scope: no rollback
      // brings it back.
      expect(frames.some((frame) => frame === "a")).toBe(false);
    });

    // O8 (G21): a callback kept from before the change, e.g. a retry
    // handler, holds the old render's data. It must start nothing.
    it("starts nothing from a run retained before the change", async () => {
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return Promise.resolve(
          calls === 1 ? START : { items: [{ id: "9", body: "neu" }] },
        );
      };
      const view = await mounted(fetcher);
      const retainedRun = action.run;
      const retainedMutate = reader.mutate;
      // The text the old session had on screen, captured by the closure.
      const captured = "stale-private-text";
      act(() =>
        operation(() =>
          view.rerender(
            <Page>
              <Chat fetcher={fetcher} />
            </Page>,
          ),
        ),
      );
      await flush();
      frames = [];
      const request = vi.fn(() => Promise.resolve({ id: "2", body: captured }));
      const commit = vi.fn((saved: Item) => {
        void retainedMutate(
          (thread) => (thread ? { items: [...thread.items, saved] } : thread),
          { revalidate: false },
        );
      });
      let outcome: OptimisticOutcome<Item> = {
        status: "done",
        result: START.items[0]!,
      };
      await act(async () => {
        outcome = await retainedRun({
          optimistic: (thread, { id }) => ({
            items: [...thread.items, { id, body: captured }],
          }),
          request,
          commit,
        });
      });
      await flush();
      expect(outcome).toEqual({ status: "discarded" });
      expect(request).not.toHaveBeenCalled();
      expect(commit).not.toHaveBeenCalled();
      expect(frames.some((frame) => frame.includes(captured))).toBe(false);
    });
  });

  it("drops every representation of the resource on a refusal with its code", async () => {
    let inboxCalls = 0;
    const inbox = () => {
      inboxCalls += 1;
      return inboxCalls === 1
        ? Promise.resolve("Posteingang")
        : new Promise<string>(() => {});
    };
    await mounted(
      () => Promise.resolve(START),
      <>
        <Other name="inbox-1" fetcher={inbox} />
        <Other name="other-1" fetcher={() => Promise.resolve("Anderes")} />
      </>,
    );
    // Anker: beide anderen Leser sind geladen.
    expect(screen.getByTestId("inbox-1")).toHaveTextContent("Posteingang");
    expect(screen.getByTestId("other-1")).toHaveTextContent("Anderes");
    const refusal = new ApiError("Forbidden", 403, {
      code: "general.permission",
    });
    let outcome: OptimisticOutcome<Item> = { status: "discarded" };
    await act(async () => {
      outcome = await send("b", () => Promise.reject(refusal), {
        keepFailed: true,
        refusal: true,
      });
    });
    expect(outcome).toEqual({
      status: "failed",
      error: refusal,
      refused: true,
    });
    expect(screen.getByTestId("inbox-1")).toHaveTextContent("leer");
    expect(screen.getByTestId("chat")).not.toHaveTextContent("b");
    expect(screen.getByTestId("other-1")).toHaveTextContent("Anderes");
  });

  it("only rolls back on a business 403 with another code", async () => {
    await mounted(
      () => Promise.resolve(START),
      <Other name="inbox-1" fetcher={() => Promise.resolve("Posteingang")} />,
    );
    // Anker: der Posteingang ist geladen.
    expect(screen.getByTestId("inbox-1")).toHaveTextContent("Posteingang");
    const business = new ApiError("Ausgeschaltet", 403, {
      code: "communication.staff_messaging_disabled",
    });
    let outcome: OptimisticOutcome<Item> = { status: "discarded" };
    await act(async () => {
      outcome = await send("b", () => Promise.reject(business), {
        refusal: true,
      });
    });
    expect(outcome).toEqual({
      status: "failed",
      error: business,
      refused: false,
    });
    expect(screen.getByTestId("chat")).toHaveTextContent(/^a$/);
    expect(screen.getByTestId("inbox-1")).toHaveTextContent("Posteingang");
  });
});

// A read can confirm the write even when its POST response was lost (#5419).
describe("message send reconciliation", () => {
  it("removes an unconfirmed bubble once the read contains its client id", async () => {
    await mounted(async () => START);
    const response = deferred<Item>();
    let outcome: Promise<OptimisticOutcome<Item>>;
    await act(async () => {
      outcome = action.run({
        optimistic: (thread, { id }) => ({
          items: [...thread.items, { id, body: "b" }],
        }),
        request: () => response.promise,
        commit: () => undefined,
        keepFailed: true,
        confirmed: (thread) =>
          thread.items.some((item) => item.id === "saved-b"),
      });
    });
    await act(async () => {
      response.reject(new TypeError("response lost"));
      await outcome;
    });
    expect(screen.getByTestId("chat")).toHaveTextContent("a|b:failed");
    await act(async () => {
      await reader.mutate(
        { items: [...START.items, { id: "saved-b", body: "b" }] },
        { revalidate: false },
      );
    });
    expect(screen.getByTestId("chat")).toHaveTextContent(/^a\|b$/);
  });
});

it("keeps an uncertain send id for retries, but not for a new acknowledged send", async () => {
  await mounted();
  const first = attempt.begin("Hallo");
  expect(first).toEqual(expect.any(String));
  expect(first.length).toBeGreaterThan(0);
  expect(attempt.begin("Hallo")).toBe(first);
  expect(attempt.begin("Anderer Text")).not.toBe(first);
  const retry = attempt.begin("Anderer Text");
  attempt.complete(first); // A late answer cannot finish the newer attempt.
  expect(attempt.begin("Anderer Text")).toBe(retry);
  attempt.complete(retry);
  expect(attempt.begin("Anderer Text")).not.toBe(retry);
});
