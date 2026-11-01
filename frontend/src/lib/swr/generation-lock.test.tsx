import { act, render, renderHook, screen } from "@testing-library/react";
import type { Session } from "next-auth";
import {
  cloneElement,
  isValidElement,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  onTestFinished,
  vi,
} from "vitest";

// The generation lock on the real SWR 2.5.1.
vi.unmock("swr");

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));

// Boundary-level test: the shared wrapper may import the package directly.
import { useSWRConfig, type SWRConfiguration, type SWRResponse } from "swr";
import { ApiError } from "~/lib/api-error";
import { _resetPersonalStorageLeave } from "~/lib/personal-storage";
import { setTestClock } from "~/test/clock";
import {
  _resetSWRFallbacksForTests,
  bumpScopeGeneration,
  isCurrentSWRValue,
  isFallbackVisible,
  preloadSWR,
  stampSWRValue,
} from "./generation";
import { swrConfig } from "./config";
import {
  useDiscardRefusedEntity,
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
  type ReaderOptions,
} from "./hooks";
import {
  _setSWRCleanupForTests,
  resetSWRScope,
  resumeSWRSession,
} from "./scope";
import { SWRSeedBoundary } from "./config-boundaries";
import { SWRScopeBoundary } from "./scope-boundary";

/**
 * Invariants (#5006):
 * - no answer of a request that started before a generation moved is shown
 *   or written after it (late answer, deduped request, preload, error
 *   retry, focus and reconnect revalidation, mutation and its rollback);
 * - no value of an older generation is shown, `fallbackData` included;
 * - both hold with the cleanup of the drops switched off: safety rests on
 *   the lock, not on the cleanup being complete.
 */

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (error: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** A fetcher that answers `first` once and `later` from then on. */
function answers(first: string, later: string) {
  let calls = 0;
  return () => {
    calls += 1;
    return Promise.resolve(calls === 1 ? first : later);
  };
}

function signedIn(user: Partial<Session["user"]> = {}) {
  return {
    data: { user: { id: "A", tenantId: 1, ...user }, expires: "2099-01-01" },
    status: "authenticated" as const,
  };
}

const FULL = (name: string) => `school-a:A:${name}`;

/** Everything each reader rendered, by its name. */
let seen: Record<string, unknown[]> = {};
const responses: Record<string, SWRResponse<string>> = {};

function Reader({
  name,
  fetcher,
  options,
}: Readonly<{
  name: string;
  fetcher: () => Promise<string>;
  options?: ReaderOptions<string>;
}>) {
  const response = useSWRAuth<string>(name, fetcher, options);
  responses[name] = response;
  (seen[name] ??= []).push(response.data);
  return <p data-testid={name}>{response.data ?? "leer"}</p>;
}

let tools: {
  discard: ReturnType<typeof useDiscardRefusedEntity>;
  matching: ReturnType<typeof useTenantMutateMatching>;
  mutate: ReturnType<typeof useTenantMutate>;
};
function Tools() {
  tools = {
    discard: useDiscardRefusedEntity(),
    mutate: useTenantMutate(),
    matching: useTenantMutateMatching(["student-7-"]),
  };
  return null;
}

function Page({ children }: Readonly<{ children?: ReactNode }>) {
  return (
    <>
      <SWRScopeBoundary expectedTenantId={1} />
      <Tools />
      {children}
    </>
  );
}

const refusal = new ApiError("Forbidden", 403, {
  code: "care.guardian_permission_denied",
});

// A session change reaches the boundary as a new render, like the
// SessionProvider's context does in the app.
type Operation = (rerender: () => void) => void;

const OPERATIONS: [string, Operation][] = [
  [
    "rights change (scope)",
    (rerender) => {
      mockUseSession.mockReturnValue(signedIn({ permissions: ["x"] }));
      rerender();
    },
  ],
  [
    // #5130/#5175: the BFF refused a request, the provider's new lookup
    // names the school the shared cookie now holds.
    "school mismatch (provider names another school)",
    (rerender) => {
      mockUseSession.mockReturnValue(signedIn({ tenantId: 2 }));
      rerender();
    },
  ],
  [
    "logout",
    (rerender) => {
      mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });
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
    "refused entity",
    () => {
      tools.discard(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-7-"),
      });
    },
  ],
  [
    "clearing matching keys",
    () => {
      void tools.matching({ clear: true });
    },
  ],
];

async function flush() {
  await act(async () => {
    for (let i = 0; i < 5; i += 1) await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

function cachedData(name: string): unknown {
  return renderHook(() => useSWRConfig()).result.current.cache.get(FULL(name))
    ?.data;
}

beforeEach(() => {
  _setSWRCleanupForTests(true);
  _resetSWRFallbacksForTests();
  resetSWRScope({ revalidate: false });
  mockUseSession.mockReturnValue(signedIn());
  seen = {};
});

afterEach(() => {
  _setSWRCleanupForTests(true);
  resumeSWRSession();
  _resetPersonalStorageLeave();
});

describe.each([
  ["with cleanup", true],
  ["lock only, cleanup off", false],
])("generation lock %s (#5006)", (_mode, cleanup) => {
  it.each(["bound", "tenant"])(
    "keeps the committed primitive through repeated failed %s mutations",
    async (kind) => {
      const name = "student-7-rollback-stamp";
      stampSWRValue(FULL(name), "vorher", 0);
      bumpScopeGeneration();
      const fetcher = vi
        .fn(() => new Promise<string>(() => {}))
        .mockResolvedValueOnce("stand");
      render(
        <Page>
          <Reader name={name} fetcher={fetcher} />
        </Page>,
      );
      await flush();
      expect(screen.getByTestId(name)).toHaveTextContent("stand");
      _setSWRCleanupForTests(cleanup);

      for (let i = 0; i < 4; i += 1) {
        const pending = deferred<string>();
        const rollbackOnError = vi.fn(() => true);
        let mutation: Promise<unknown> = Promise.resolve();
        act(() => {
          const options = {
            optimisticData:
              i % 2 === 0 ? `optimistisch-${i}` : () => `optimistisch-${i}`,
            ...(i % 2 === 0 ? {} : { rollbackOnError }),
            revalidate: false,
          };
          mutation = (
            kind === "bound"
              ? responses[name]!.mutate(pending.promise, options)
              : tools.mutate(name, pending.promise, options)
          ).catch(() => undefined);
        });
        expect(screen.getByTestId(name)).toHaveTextContent(`optimistisch-${i}`);
        await act(async () => {
          pending.reject(new Error("Speichern fehlgeschlagen"));
          await mutation;
        });
        expect(screen.getByTestId(name)).toHaveTextContent("stand");
        if (i % 2 !== 0) expect(rollbackOnError).toHaveBeenCalledTimes(1);
      }

      const update = vi.fn((current: unknown) => `${String(current)}-neu`);
      await act(async () => {
        if (kind === "bound") {
          await responses[name]!.mutate(update, { revalidate: false });
        } else {
          await tools.mutate(name, update, { revalidate: false });
        }
      });
      expect(update).toHaveBeenCalledWith("stand");
      expect(screen.getByTestId(name)).toHaveTextContent("stand-neu");
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );

  describe.each(OPERATIONS.slice(4))(
    "unseen fallback after %s",
    (_name, operation) => {
      it.each(["fallbackData", "provider"])(
        "rejects an invalidated %s",
        async (source) => {
          const view = render(<Page />);
          _setSWRCleanupForTests(cleanup);
          act(() => operation(() => view.rerender(<Page />)));
          const answer = deferred<string>();
          const fetcher = vi.fn(() => answer.promise);
          const key = "student-7-unseen";
          const kept = "student-8-unseen";
          // A provider fallback reaches readers only through SWRSeedBoundary
          // (#5007); fallbackData is a reader option.
          const options =
            source === "fallbackData" ? { fallbackData: "veraltet" } : {};
          const readers = (
            <>
              <Reader name={key} fetcher={fetcher} options={options} />
              <Reader name={kept} fetcher={fetcher} options={options} />
            </>
          );
          view.rerender(
            <Page>
              {source === "fallbackData" ? (
                readers
              ) : (
                <SWRSeedBoundary
                  seed={{
                    fallback: {
                      [FULL(key)]: "veraltet",
                      [FULL(kept)]: "veraltet",
                    },
                    cacheData: {},
                  }}
                >
                  {readers}
                </SWRSeedBoundary>
              )}
            </Page>,
          );
          // Anker: ein nicht verworfener, ebenfalls neuer Key zeigt den Fallback.
          expect(screen.getByTestId(kept)).toHaveTextContent("veraltet");
          expect(screen.getByTestId(key)).toHaveTextContent("leer");
          expect(seen[key]).not.toContain("veraltet");
          answer.resolve("frisch");
          await flush();
          expect(screen.getByTestId(key)).toHaveTextContent("frisch");
          expect(fetcher).toHaveBeenCalledTimes(2);
          expect(isFallbackVisible(FULL(key))).toBe(true);
        },
      );
    },
  );

  it("keeps an unseen fallback key named in except", async () => {
    const view = render(<Page />);
    _setSWRCleanupForTests(cleanup);
    act(() => {
      void tools.matching({ clear: true, except: ["student-7-unseen-kept"] });
    });
    const fetcher = vi.fn(() => new Promise<string>(() => {}));
    view.rerender(
      <Page>
        <Reader
          name="student-7-unseen-kept"
          fetcher={fetcher}
          options={{ fallbackData: "behalten" }}
        />
        <Reader
          name="student-7-unseen-dropped"
          fetcher={fetcher}
          options={{ fallbackData: "veraltet" }}
        />
      </Page>,
    );
    // Anker: der ausgenommene Key zeigt seinen Fallback beim ersten Lesen.
    expect(screen.getByTestId("student-7-unseen-kept")).toHaveTextContent(
      "behalten",
    );
    expect(screen.getByTestId("student-7-unseen-dropped")).toHaveTextContent(
      "leer",
    );
    expect(seen["student-7-unseen-dropped"]).not.toContain("veraltet");
    await flush();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("rejects a tenant write from a passive effect after the boundary reset", async () => {
    const fetcher = answers("stand", "neu");
    const writes: unknown[] = [];
    function Writer({ write }: Readonly<{ write: boolean }>) {
      const mutate = useTenantMutate();
      const { data } = useSWRAuth("student-7-effect", fetcher);
      useEffect(() => {
        if (write && data)
          void mutate("student-7-effect-copy", data, {
            revalidate: false,
          }).then((value) => writes.push(value));
      }, [write, data, mutate]);
      return <p data-testid="effect-source">{data ?? "leer"}</p>;
    }
    const view = render(
      <Page>
        <Writer write={false} />
      </Page>,
    );
    await flush();
    // Anker: der Effekt wird mit den Daten vor dem Rechtewechsel gerendert.
    expect(screen.getByTestId("effect-source")).toHaveTextContent("stand");
    _setSWRCleanupForTests(cleanup);
    mockUseSession.mockReturnValue(signedIn({ permissions: ["x"] }));
    view.rerender(
      <Page>
        <Writer write />
      </Page>,
    );
    await flush();
    expect(screen.getByTestId("effect-source")).toHaveTextContent("neu");
    expect(cachedData("student-7-effect-copy")).not.toBe("stand");
    expect(writes).not.toContain("stand");
  });

  it.each(["value", "updater", "promise"] as const)(
    "binds a retained %s mutation to its captured key",
    async (kind) => {
      const oldKey = `student-7-retained-${kind}-${cleanup}`;
      const newKey = `student-8-current-${kind}-${cleanup}`;
      const fetcher = vi.fn(() => Promise.resolve("stand"));
      const view = render(
        <Page>
          <Reader name={oldKey} fetcher={fetcher} />
        </Page>,
      );
      await flush();
      expect(screen.getByTestId(oldKey)).toHaveTextContent("stand");
      const retained = responses[oldKey]!.mutate;
      _setSWRCleanupForTests(cleanup);
      view.rerender(
        <Page>
          <Reader name={newKey} fetcher={fetcher} />
        </Page>,
      );
      await flush();
      expect(screen.getByTestId(newKey)).toHaveTextContent("stand");

      await act(async () => {
        await retained(
          kind === "value"
            ? "gespeichert"
            : kind === "updater"
              ? (current) => `${current}-gespeichert`
              : Promise.resolve("gespeichert"),
          { revalidate: false },
        );
      });

      expect(cachedData(newKey)).toBe("stand");
      expect(cachedData(oldKey)).toBe(
        kind === "updater" ? "stand-gespeichert" : "gespeichert",
      );
      expect(screen.getByTestId(newKey)).toHaveTextContent("stand");
      expect(seen[newKey]).toEqual(expect.arrayContaining(["stand"]));
      expect(seen[newKey]).not.toContain("gespeichert");
      expect(seen[newKey]).not.toContain("stand-gespeichert");
      expect(fetcher).toHaveBeenCalledTimes(2);
    },
  );

  describe.each(OPERATIONS)("%s", (_name, operation) => {
    async function run(ui: ReactNode, before: () => Promise<void> | void) {
      const view = render(ui);
      await before();
      _setSWRCleanupForTests(cleanup);
      act(() =>
        operation(() =>
          view.rerender(isValidElement(ui) ? cloneElement(ui) : ui),
        ),
      );
      return view;
    }

    it.each(["value", "updater", "promise"] as const)(
      "rejects a retained %s mutation after its generation ended",
      async (kind) => {
        const name = `student-7-retained-generation-${kind}`;
        let retained: SWRResponse<string>["mutate"];
        await run(
          <Page>
            <Reader name={name} fetcher={answers("stand", "neu")} />
          </Page>,
          async () => {
            await flush();
            expect(screen.getByTestId(name)).toHaveTextContent("stand");
            retained = responses[name]!.mutate;
          },
        );
        await flush();
        expect(responses[name]!.mutate).not.toBe(retained!);
        const before = cachedData(name);
        const updater = vi.fn(() => "alt");
        await act(async () => {
          await retained!(
            kind === "value"
              ? "alt"
              : kind === "updater"
                ? updater
                : Promise.resolve("alt"),
            { optimisticData: "alt", revalidate: false },
          );
        });
        await flush();
        expect(cachedData(name)).toBe(before);
        expect(seen[name]).not.toContain("alt");
        expect(updater).not.toHaveBeenCalled();
        if (responses[name]!.data === "neu") {
          await act(async () => {
            await responses[name]!.mutate("aktuell", { revalidate: false });
          });
          expect(screen.getByTestId(name)).toHaveTextContent("aktuell");
        }
      },
    );

    it.each(["primitive", "shared object"] as const)(
      "keeps the fresh stamp when an old mutation resolves to the same %s",
      async (kind) => {
        const name = `student-7-shared-${kind}`;
        const fresh = kind === "primitive" ? 0 : { count: 0 };
        const pending = deferred<typeof fresh>();
        let response: SWRResponse<typeof fresh>;
        let saving: Promise<unknown>;
        function SharedReader() {
          response = useSWRAuth(name, () => Promise.resolve(fresh));
          return <p data-testid={name}>{JSON.stringify(response.data)}</p>;
        }
        const view = await run(
          <Page>
            <SharedReader />
          </Page>,
          async () => {
            await flush();
            expect(screen.getByTestId(name)).toHaveTextContent(
              JSON.stringify(fresh),
            );
            saving = response.mutate(pending.promise, { revalidate: false });
          },
        );
        await flush();
        // Resume only operations that removed access. The other rows
        // commit in the generation raised by the operation itself.
        if (
          _name === "logout" ||
          _name === "school mismatch (provider names another school)" ||
          _name === "another account in another tab"
        ) {
          resumeSWRSession();
          mockUseSession.mockReturnValue(signedIn({ permissions: ["fresh"] }));
          view.rerender(
            <Page>
              <SharedReader />
            </Page>,
          );
          await flush();
        }
        // SWR itself suppresses fetch writes while the older mutation is
        // pending when cleanup is off. Commit a fresh mutation instead;
        // it shares the same stamp registry as a fresh fetch result.
        await act(async () => {
          await response.mutate(fresh, { revalidate: false });
        });
        view.rerender(
          <Page>
            <SharedReader />
          </Page>,
        );
        expect(screen.getByTestId(name)).toHaveTextContent(
          JSON.stringify(fresh),
        );
        expect(isCurrentSWRValue(FULL(name), fresh)).toBe(true);
        await act(async () => {
          pending.resolve(fresh);
          await saving!;
        });
        expect(isCurrentSWRValue(FULL(name), fresh)).toBe(true);
        view.rerender(
          <Page>
            <SharedReader />
          </Page>,
        );
        expect(screen.getByTestId(name)).toHaveTextContent(
          JSON.stringify(fresh),
        );
      },
    );

    it("shows, logs and retries no late error", async () => {
      const answer = deferred<string>();
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1 ? answer.promise : Promise.resolve("neu");
      };
      const onError = vi.fn();
      // onErrorRetry is no reader option (#5007): watch the shared one.
      const onErrorRetry = vi.fn();
      const configuredRetry = swrConfig.onErrorRetry;
      swrConfig.onErrorRetry = onErrorRetry;
      onTestFinished(() => {
        swrConfig.onErrorRetry = configuredRetry;
      });
      const ui = (
        <Page>
          <Reader
            name="student-7-late-error"
            fetcher={fetcher}
            options={{ onError }}
          />
        </Page>
      );
      await run(ui, () => {
        // Anker: der Abruf läuft.
        expect(calls).toBe(1);
      });

      answer.reject(
        new ApiError("Forbidden", 403, { code: "general.permission" }),
      );
      await flush();

      expect(responses["student-7-late-error"]!.error).toBeUndefined();
      expect(onError).not.toHaveBeenCalled();
      expect(onErrorRetry).not.toHaveBeenCalled();
    });

    it("shows and writes no late answer", async () => {
      const answer = deferred<string>();
      let calls = 0;
      // Only the request from before the bump answers with old data; a
      // request of the new generation gets its own answer.
      const fetcher = () => {
        calls += 1;
        return calls === 1 ? answer.promise : Promise.resolve("neu");
      };
      const ui = (
        <Page>
          <Reader name="student-7-late" fetcher={fetcher} />
        </Page>
      );
      await run(ui, () => {
        // Anker: der Abruf läuft.
        expect(screen.getByTestId("student-7-late")).toHaveTextContent("leer");
      });

      answer.resolve("alt");
      await flush();

      expect(seen["student-7-late"]).not.toContain("alt");
      expect(cachedData("student-7-late")).not.toBe("alt");
    });

    it.each(["value", "updater", "promise", "revalidate"] as const)(
      "rejects a retained tenant %s mutation after the bump",
      async (kind) => {
        const key = `student-7-tenant-${kind}`;
        const fetcher = vi.fn(answers("stand", "neu"));
        let retained = tools?.mutate;
        await run(
          <Page>
            <Reader name={key} fetcher={fetcher} />
          </Page>,
          async () => {
            await flush();
            // Anker: der Leser zeigt Daten der Ursprungsgeneration.
            expect(screen.getByTestId(key)).toHaveTextContent("stand");
            retained = tools.mutate;
          },
        );
        await flush();
        seen = {};
        const calls = fetcher.mock.calls.length;
        const updater = vi.fn(() => "veraltet");
        let result: unknown;
        await act(async () => {
          result = await retained(
            key,
            kind === "value"
              ? "veraltet"
              : kind === "updater"
                ? updater
                : kind === "promise"
                  ? Promise.resolve("veraltet")
                  : undefined,
            { revalidate: false },
          );
        });
        await flush();
        expect(result).toBeUndefined();
        expect(updater).not.toHaveBeenCalled();
        expect(fetcher).toHaveBeenCalledTimes(calls);
        expect(cachedData(key)).not.toBe("veraltet");
        expect(seen[key] ?? []).not.toContain("veraltet");
      },
    );

    it("shows no deduped answer to a reader that joins after the bump", async () => {
      const answer = deferred<string>();
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1 ? answer.promise : Promise.resolve("neu");
      };
      const first = (
        <Page>
          <Reader name="student-7-dedupe" fetcher={fetcher} />
        </Page>
      );
      const view = await run(first, () => {
        expect(calls).toBe(1);
      });
      // A second reader of the same key takes over the request in flight.
      view.rerender(
        <Page>
          <Reader name="student-7-dedupe" fetcher={fetcher} />
          <Reader name="student-7-dedupe" fetcher={fetcher} />
        </Page>,
      );

      answer.resolve("alt");
      await flush();

      expect(seen["student-7-dedupe"]).not.toContain("alt");
      expect(cachedData("student-7-dedupe")).not.toBe("alt");
    });

    it("runs no error retry scheduled before the bump", async () => {
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1
          ? Promise.reject(new Error("vorübergehend"))
          : Promise.resolve("neu");
      };
      // The retry SWR schedules; fired by hand after the drop, so its timing
      // cannot race the test.
      let scheduled: (() => void) | null = null;
      const retry: NonNullable<SWRConfiguration["onErrorRetry"]> = (
        _error,
        _key,
        _config,
        revalidate,
        options,
      ) => {
        scheduled = () => void revalidate(options);
      };
      // onErrorRetry is no reader option (#5007): the shared configuration
      // schedules every retry, so it captures this one.
      const configuredRetry = swrConfig.onErrorRetry;
      swrConfig.onErrorRetry = retry;
      onTestFinished(() => {
        swrConfig.onErrorRetry = configuredRetry;
      });
      const ui = (
        <Page>
          <Reader
            name="student-7-retry"
            fetcher={fetcher}
            options={{ dedupingInterval: 0 }}
          />
        </Page>
      );
      await run(ui, async () => {
        await flush();
        // Anker: der erste Abruf ist gescheitert, der Retry ist geplant.
        expect(calls).toBe(1);
        expect(scheduled).not.toBeNull();
      });
      // Whatever the drop itself loads again (a rights change reloads the
      // mounted reader) has started now.
      await flush();
      const afterDrop = calls;
      const fire = scheduled as (() => void) | null;

      act(() => fire?.());
      await flush();

      // The retry the first error scheduled sends no request.
      expect(calls).toBe(afterDrop);
    });

    it.each([
      ["after a remount", "remount"],
      ["in a second hook", "second"],
      ["with a new fallback prop", "prop"],
    ])("shows no dropped fallback %s", async (_case, variant) => {
      const never = () => new Promise<string>(() => {});
      const reader = (fallback: string, name = "student-7-fb") => (
        <Reader
          name={name}
          fetcher={never}
          options={{ fallbackData: fallback }}
        />
      );
      const view = await run(<Page>{reader("vorher")}</Page>, () => {
        // Anker: vor dem Wechsel steht der Fallback.
        expect(screen.getByTestId("student-7-fb")).toHaveTextContent("vorher");
      });
      seen = {};
      if (variant === "remount") {
        view.rerender(<Page />);
        view.rerender(<Page>{reader("vorher")}</Page>);
      } else if (variant === "second") {
        view.rerender(
          <Page>
            {reader("vorher")}
            {reader("vorher", "student-7-fb")}
          </Page>,
        );
      } else {
        view.rerender(<Page>{reader("anders")}</Page>);
      }
      await flush();

      expect(seen["student-7-fb"] ?? []).not.toContain("vorher");
      expect(seen["student-7-fb"] ?? []).not.toContain("anders");
      expect(screen.queryByText("vorher")).toBeNull();
      expect(screen.queryByText("anders")).toBeNull();
    });

    it("shows no optimistic value of a mutation without populateCache", async () => {
      const ui = (
        <Page>
          <Reader
            name="student-7-optimistic"
            fetcher={answers("stand", "neu")}
          />
        </Page>
      );
      const saved = deferred<string>();
      await run(ui, async () => {
        await flush();
        void responses["student-7-optimistic"]!.mutate(saved.promise, {
          optimisticData: "optimistisch",
          populateCache: false,
          revalidate: false,
        });
        await flush();
        // Anker: der optimistische Wert steht.
        expect(screen.getByTestId("student-7-optimistic")).toHaveTextContent(
          "optimistisch",
        );
      });
      seen = {};
      await flush();

      expect(seen["student-7-optimistic"] ?? []).not.toContain("optimistisch");
      saved.resolve("gespeichert");
      await flush();
      expect(seen["student-7-optimistic"] ?? []).not.toContain("optimistisch");
    });

    it("shows a deeply equal answer of the new generation", async () => {
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return Promise.resolve(JSON.stringify({ rows: ["a"] }));
      };
      let equalResponse: SWRResponse<{ rows: string[] }> | undefined;
      function ObjectReader() {
        const response = useSWRAuth<{ rows: string[] }>(
          "student-7-equal",
          () =>
            fetcher().then((text) => JSON.parse(text) as { rows: string[] }),
          { dedupingInterval: 0 },
        );
        equalResponse = response;
        return (
          <p data-testid="student-7-equal">
            {response.data ? response.data.rows.join(",") : "leer"}
          </p>
        );
      }
      const ui = (
        <Page>
          <ObjectReader />
        </Page>
      );
      await run(ui, async () => {
        await flush();
        // Anker: geladen.
        expect(screen.getByTestId("student-7-equal")).toHaveTextContent("a");
      });
      await flush();
      const before = calls;
      // Wherever a session remains, load the key again explicitly: the
      // same content of the new generation must be shown.
      act(() => {
        void equalResponse?.mutate();
      });
      await flush();

      if (calls > before) {
        expect(screen.getByTestId("student-7-equal")).toHaveTextContent("a");
      } else {
        // Logout and the other tab's signal load nothing: no session yet.
        expect(screen.getByTestId("student-7-equal")).toHaveTextContent("leer");
      }
    });

    it("resolves no mutate() to a value of the generation before", async () => {
      // Requests after the drop never answer, so the cache keeps (without
      // cleanup) or loses (with it) the old value; neither may come back.
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1
          ? Promise.resolve("stand")
          : new Promise<string>(() => {});
      };
      await run(
        <Page>
          <Reader
            name="student-7-result"
            fetcher={fetcher}
            options={{ dedupingInterval: 0 }}
          />
        </Page>,
        async () => {
          await flush();
          // Anker: geladen.
          expect(screen.getByTestId("student-7-result")).toHaveTextContent(
            "stand",
          );
        },
      );
      let result: unknown = "offen";
      await act(async () => {
        result = await responses["student-7-result"]!.mutate(
          (current: string | undefined) => current,
          { revalidate: false },
        );
      });

      expect(result).not.toBe("stand");
    });

    it("shows no failure of the generation before", async () => {
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1
          ? Promise.reject(new Error("alter Fehler"))
          : new Promise<string>(() => {});
      };
      function ErrorReader() {
        const response = useSWRAuth<string>("student-7-failure", fetcher, {
          shouldRetryOnError: false,
          dedupingInterval: 0,
        });
        return (
          <p data-testid="student-7-failure">
            {response.error ? String((response.error as Error).message) : "–"}
          </p>
        );
      }
      await run(
        <Page>
          <ErrorReader />
        </Page>,
        async () => {
          await flush();
          // Anker: der Fehler steht.
          expect(screen.getByTestId("student-7-failure")).toHaveTextContent(
            "alter Fehler",
          );
        },
      );
      await flush();

      expect(screen.queryByText("alter Fehler")).toBeNull();
    });

    it("shows no fallbackData of the generation before", async () => {
      const ui = (
        <Page>
          <Reader
            name="student-7-fallback"
            fetcher={() => new Promise<string>(() => {})}
            options={{ fallbackData: "vorher" }}
          />
        </Page>
      );
      await run(ui, () => {
        // Anker: vor dem Wechsel steht der Fallback.
        expect(screen.getByTestId("student-7-fallback")).toHaveTextContent(
          "vorher",
        );
      });
      seen = {};
      await flush();

      expect(seen["student-7-fallback"] ?? []).not.toContain("vorher");
      expect(screen.queryByText("vorher")).toBeNull();
    });

    it("hands no preload of the generation before to a later reader", async () => {
      const answer = deferred<string>();
      const ui = <Page />;
      let preloaded: Promise<string> | undefined;
      const view = await run(ui, () => {
        preloaded = preloadSWR(FULL("student-7-preload"), () => answer.promise);
        // Anker: der Preload läuft.
        expect(preloaded).toBeInstanceOf(Promise);
      });
      answer.resolve("alt");
      await act(async () => {
        await preloaded?.catch(() => undefined);
      });

      view.rerender(
        <Page>
          <Reader
            name="student-7-preload"
            fetcher={() => Promise.resolve("neu")}
          />
        </Page>,
      );
      await flush();

      expect(seen["student-7-preload"]).not.toContain("alt");
    });

    it("keeps neither the late answer nor the rollback of a mutation", async () => {
      const ui = (
        <Page>
          <Reader name="student-7-saved" fetcher={answers("stand", "neu")} />
          <Reader name="student-7-rolled" fetcher={answers("stand", "neu")} />
        </Page>
      );
      const saved = deferred<string>();
      const rolled = deferred<string>();
      let saving: Promise<unknown> = Promise.resolve();
      let rolling: Promise<unknown> = Promise.resolve();
      await run(ui, async () => {
        await flush();
        // Anker: beide Leser zeigen den Stand vor der Mutation.
        expect(screen.getByTestId("student-7-saved")).toHaveTextContent(
          "stand",
        );
        saving = responses["student-7-saved"]!.mutate(saved.promise, {
          revalidate: false,
        });
        rolling = responses["student-7-rolled"]!.mutate(rolled.promise, {
          optimisticData: "optimistisch",
          rollbackOnError: true,
          revalidate: false,
        }).catch(() => undefined);
        await flush();
        expect(screen.getByTestId("student-7-rolled")).toHaveTextContent(
          "optimistisch",
        );
      });
      seen = {};

      saved.resolve("gespeichert");
      rolled.reject(new Error("Speichern fehlgeschlagen"));
      await act(async () => {
        await saving;
        await rolling;
      });
      await flush();

      expect(seen["student-7-saved"] ?? []).not.toContain("gespeichert");
      expect(cachedData("student-7-saved")).not.toBe("gespeichert");
      expect(seen["student-7-rolled"] ?? []).not.toContain("stand");
    });

    it.each([
      ["reconnect", () => globalThis.dispatchEvent(new Event("online"))],
      ["focus", () => globalThis.dispatchEvent(new Event("focus"))],
    ])(
      "shows no answer of a %s revalidation from before",
      async (_event, trigger) => {
        let current = deferred<string>();
        let late = current;
        let calls = 0;
        const fetcher = () => {
          calls += 1;
          return calls === 1 ? Promise.resolve("stand") : current.promise;
        };
        const ui = (
          <Page>
            <Reader
              name="student-7-event"
              fetcher={fetcher}
              options={{
                dedupingInterval: 0,
                revalidateOnFocus: true,
                focusThrottleInterval: 0,
              }}
            />
          </Page>
        );
        await run(ui, async () => {
          await flush();
          // Focus revalidation is throttled against Date.now(), which the
          // test clock freezes.
          setTestClock("2026-09-09T12:00:01+02:00");
          act(() => trigger());
          await flush();
          // Anker: die Revalidierung läuft.
          expect(calls).toBe(2);
          late = current;
          // Requests of the new generation get their own answer.
          current = deferred<string>();
        });

        late.resolve("alt");
        await flush();

        expect(seen["student-7-event"]).not.toContain("alt");
        expect(cachedData("student-7-event")).not.toBe("alt");
      },
    );
  });
});

describe("what the lock leaves alone (#5006)", () => {
  it("presents a load failure once when the error effect depends on mutate", async () => {
    const failure = new ApiError("Unavailable", 503);
    const fetcher = vi.fn(() => Promise.reject(failure));
    const present = vi.fn();
    function LoadErrorReader() {
      const { error, mutate } = useSWRAuth("load-error", fetcher, {
        shouldRetryOnError: false,
      });
      const [presentation, setPresentation] = useState<object | null>(null);
      useEffect(() => {
        if (!error) return;
        present(error);
        // Bound the broken loop so the regression fails without hanging.
        if (present.mock.calls.length < 3) setPresentation({ retry: mutate });
      }, [error, mutate]);
      return <p>{presentation ? "Fehler angezeigt" : "Lädt"}</p>;
    }

    render(
      <Page>
        <LoadErrorReader />
      </Page>,
    );
    await flush();

    expect(screen.getByText("Fehler angezeigt")).toBeInTheDocument();
    expect(present).toHaveBeenCalledExactlyOnceWith(failure);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("keeps a key named in `except` and its late answer", async () => {
    const answer = deferred<string>();
    let calls = 0;
    const fetcher = () => {
      calls += 1;
      return calls === 1 ? Promise.resolve("stand") : answer.promise;
    };
    render(
      <Page>
        <Reader
          name="student-7-kept"
          fetcher={fetcher}
          options={{ dedupingInterval: 0 }}
        />
      </Page>,
    );
    await flush();
    const retained = responses["student-7-kept"]!.mutate;
    const tenantMutate = tools.mutate;
    act(() => {
      void responses["student-7-kept"]!.mutate();
    });

    act(() => {
      void tools.matching({ clear: true, except: ["student-7-kept"] });
    });
    answer.resolve("neu");
    await flush();

    // Anker: der Leser zeigt die Antwort, die nach dem Stand kam.
    expect(screen.getByTestId("student-7-kept")).toHaveTextContent("neu");
    expect(responses["student-7-kept"]!.mutate).toBe(retained);
    await act(async () => {
      await retained("aktuell", { revalidate: false });
    });
    expect(screen.getByTestId("student-7-kept")).toHaveTextContent("aktuell");
    await act(async () => {
      await tenantMutate("student-7-kept", "erhalten", { revalidate: false });
    });
    expect(screen.getByTestId("student-7-kept")).toHaveTextContent("erhalten");
  });

  it("revalidates on a bare mutate() and keeps the data on screen", async () => {
    // SWR counts arguments: a locked mutate must not turn mutate() into a
    // write of undefined, which emptied lists while they reloaded.
    const later = deferred<string>();
    let calls = 0;
    const fetcher = () => {
      calls += 1;
      return calls === 1 ? Promise.resolve("stand") : later.promise;
    };
    render(
      <Page>
        <Reader
          name="student-7-bare"
          fetcher={fetcher}
          // Like the request lists: nothing else bridges an emptied entry.
          options={{ dedupingInterval: 0, keepPreviousData: false }}
        />
      </Page>,
    );
    await flush();
    // Anker: geladen.
    expect(screen.getByTestId("student-7-bare")).toHaveTextContent("stand");

    act(() => {
      void responses["student-7-bare"]!.mutate();
    });
    await flush();

    expect(calls).toBe(2);
    expect(screen.getByTestId("student-7-bare")).toHaveTextContent("stand");
    later.resolve("neu");
    expect(await screen.findByText("neu")).toBeInTheDocument();
  });

  it("loads nothing again without a generation change", async () => {
    const fetcher = vi.fn(() => Promise.resolve("stand"));
    const ui = (
      <Page>
        <Reader name="student-7-steady" fetcher={fetcher} />
      </Page>
    );
    const view = render(ui);
    await flush();
    // Anker: geladen.
    expect(screen.getByTestId("student-7-steady")).toHaveTextContent("stand");
    const retained = responses["student-7-steady"]!.mutate;

    view.rerender(ui);
    act(() => {
      // Another child's refusal is no change for this key.
      tools.discard(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-8-"),
      });
    });
    view.rerender(ui);
    await flush();

    expect(screen.getByTestId("student-7-steady")).toHaveTextContent("stand");
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(responses["student-7-steady"]!.mutate).toBe(retained);
  });
});

describe.each([
  ["with cleanup", true],
  ["lock only, cleanup off", false],
])("a running mutation and a bump %s (#5223)", (_mode, cleanup) => {
  // The first request answers, every later one never: a write the test sees
  // can only come from the mutation.
  function loadsOnce(value: string) {
    let calls = 0;
    return vi.fn(() => {
      calls += 1;
      return calls === 1
        ? Promise.resolve(value)
        : new Promise<string>(() => {});
    });
  }

  function start(kind: "bound" | "tenant", name: string, data: unknown) {
    return kind === "bound"
      ? responses[name]!.mutate(data as Promise<string>, { revalidate: false })
      : tools.mutate(name, data, { revalidate: false });
  }

  it.each(["bound", "tenant"] as const)(
    "keeps the result of a running %s mutation across a request bump",
    async (kind) => {
      const name = `student-7-request-bump-${kind}`;
      const fetcher = loadsOnce("v0");
      render(
        <Page>
          <Reader
            name={name}
            fetcher={fetcher}
            options={{ dedupingInterval: 0 }}
          />
        </Page>,
      );
      await flush();
      // Anker: der Leser zeigt den Stand vor der Mutation.
      expect(screen.getByTestId(name)).toHaveTextContent("v0");
      _setSWRCleanupForTests(cleanup);
      const saved = deferred<string>();
      let saving!: Promise<unknown>;
      act(() => {
        saving = start(kind, name, saved.promise);
      });
      // Revalidating matching keys is no drop: their values stay valid.
      act(() => {
        void tools.matching();
      });
      let result: unknown;
      await act(async () => {
        saved.resolve("v1");
        result = await saving;
      });
      await flush();

      expect(result).toBe("v1");
      expect(cachedData(name)).toBe("v1");
      expect(screen.getByTestId(name)).toHaveTextContent("v1");
    },
  );

  // G25: whether a run counts is bound to the run, never to its result
  // being equal to a value stamped since (#5460 review).
  describe.each([
    ["0", 0],
    ["null", null],
    ["undefined", undefined],
  ] as const)("a result equal to the fresh value %s", (_label, value) => {
    it.each(["bound", "tenant"] as const)(
      "drops the result of a %s mutation that started before a drop",
      async (kind) => {
        const name = `student-7-equal-result-${kind}`;
        render(
          <Page>
            <Reader
              name={name}
              fetcher={loadsOnce("v0")}
              options={{ dedupingInterval: 0 }}
            />
          </Page>,
        );
        await flush();
        // Anker: der Leser zeigt den Stand vor beiden Mutationen.
        expect(screen.getByTestId(name)).toHaveTextContent("v0");
        _setSWRCleanupForTests(cleanup);
        const late = deferred<unknown>();
        let old!: Promise<unknown>;
        act(() => {
          old = start(kind, name, late.promise);
        });
        act(() => {
          void tools.matching({ clear: true });
        });
        // A mutation of the new generation writes the same value (a bound
        // mutate of `undefined` clears; useTenantMutate would revalidate).
        await act(async () => {
          await responses[name]!.mutate(value as never, { revalidate: false });
        });
        let result: unknown = "offen";
        await act(async () => {
          late.resolve(value);
          result = await old;
        });
        await flush();

        expect(result).toBeUndefined();
        if (value !== undefined) expect(cachedData(name)).toBe(value);
      },
    );
  });

  describe.each(OPERATIONS)("%s", (_operationName, operation) => {
    it.each(["bound", "tenant"] as const)(
      "drops the result of a running %s mutation",
      async (kind) => {
        const name = `student-7-drop-bump-${kind}`;
        const ui = (
          <Page>
            <Reader
              name={name}
              fetcher={loadsOnce("v0")}
              options={{ dedupingInterval: 0 }}
            />
          </Page>
        );
        const view = render(ui);
        await flush();
        // Anker: der Leser zeigt den Stand vor der Mutation.
        expect(screen.getByTestId(name)).toHaveTextContent("v0");
        _setSWRCleanupForTests(cleanup);
        const saved = deferred<string>();
        let saving!: Promise<unknown>;
        act(() => {
          saving = start(kind, name, saved.promise);
        });
        act(() => operation(() => view.rerender(cloneElement(ui))));
        seen = {};
        let result: unknown;
        await act(async () => {
          saved.resolve("v1");
          result = await saving;
        });
        await flush();

        expect(result).toBeUndefined();
        expect(cachedData(name)).not.toBe("v1");
        expect(seen[name] ?? []).not.toContain("v1");
      },
    );
  });
});

describe.each([
  ["with cleanup", true],
  ["lock only, cleanup off", false],
])("the stamp of the committed primitive %s (#5485)", (_mode, cleanup) => {
  // After a boundary released stamps, a value without one no longer counts
  // (G22): the committed value must keep its own stamp.
  async function committed(name: string) {
    stampSWRValue(FULL(name), "vorher", 0);
    bumpScopeGeneration();
    const fetcher = vi
      .fn(() => new Promise<string>(() => {}))
      .mockResolvedValueOnce("stand");
    render(
      <Page>
        <Reader name={name} fetcher={fetcher} />
      </Page>,
    );
    await flush();
    // Anker: der bestätigte Wert steht.
    expect(screen.getByTestId(name)).toHaveTextContent("stand");
    _setSWRCleanupForTests(cleanup);
    return fetcher;
  }

  async function expectCommitted(name: string, fetcher: () => unknown) {
    expect(screen.getByTestId(name)).toHaveTextContent("stand");
    const update = vi.fn((current: unknown) => `${String(current)}-neu`);
    await act(async () => {
      await responses[name]!.mutate(update, { revalidate: false });
    });
    expect(update).toHaveBeenCalledWith("stand");
    expect(screen.getByTestId(name)).toHaveTextContent("stand-neu");
    expect(fetcher).toHaveBeenCalledTimes(1);
  }

  it("keeps it through results that populate nothing", async () => {
    const name = "student-7-unpopulated";
    const fetcher = await committed(name);
    for (let i = 0; i < 5; i += 1) {
      await act(async () => {
        await responses[name]!.mutate(Promise.resolve(`ergebnis-${i}`), {
          populateCache: false,
          revalidate: false,
        });
      });
    }

    await expectCommitted(name, fetcher);
  });

  it("keeps it through overlapping optimistic mutations", async () => {
    const name = "student-7-overlapping";
    const fetcher = await committed(name);
    const pending = Array.from({ length: 5 }, () => deferred<string>());
    const runs: Promise<unknown>[] = [];
    act(() => {
      pending.forEach((answer, i) => {
        runs.push(
          responses[name]!.mutate(answer.promise, {
            optimisticData: `optimistisch-${i}`,
            rollbackOnError: true,
            revalidate: false,
          }).catch(() => undefined),
        );
      });
    });
    expect(screen.getByTestId(name)).toHaveTextContent("optimistisch-4");

    await act(async () => {
      pending[4]!.reject(new Error("Speichern fehlgeschlagen"));
      await runs[4];
    });
    await flush();
    expect(screen.getByTestId(name)).toHaveTextContent("stand");
    await act(async () => {
      for (const answer of pending.slice(0, 4))
        answer.reject(new Error("Speichern fehlgeschlagen"));
      await Promise.all(runs);
    });
    await flush();

    await expectCommitted(name, fetcher);
  });
});

describe("callback identity across bumps of other keys (#5223)", () => {
  it("keeps the useTenantMutate callback across a request bump", async () => {
    const name = "student-7-identity";
    const ui = (
      <Page>
        <Reader name={name} fetcher={() => Promise.resolve("stand")} />
      </Page>
    );
    const view = render(ui);
    await flush();
    // Anker: geladen.
    expect(screen.getByTestId(name)).toHaveTextContent("stand");
    const before = tools.mutate;

    act(() => {
      void tools.matching();
    });
    await flush();
    view.rerender(cloneElement(ui));

    expect(tools.mutate).toBe(before);
    await act(async () => {
      await tools.mutate(name, "neu", { revalidate: false });
    });
    expect(screen.getByTestId(name)).toHaveTextContent("neu");
  });

  it("hands out a new useTenantMutate callback after a data bump", async () => {
    // The callback takes any key, so a drop of any key ends it (G21).
    const view = render(<Page />);
    const before = tools.mutate;
    act(() => {
      tools.discard(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-8-"),
      });
    });
    view.rerender(<Page />);

    expect(tools.mutate).not.toBe(before);
  });
});

describe.each([
  ["with cleanup", true],
  ["lock only, cleanup off", false],
])("released stamps %s (#5223)", (_mode, cleanup) => {
  describe.each(OPERATIONS)("%s", (_name, operation) => {
    it.each(["object", "primitive"] as const)(
      "lets no reachable %s of the generation before count again",
      async (kind) => {
        const name = `student-7-released-${kind}`;
        // Held by the test: still reachable after the boundary.
        const old = kind === "object" ? { label: "alt" } : "alt";
        let calls = 0;
        const fetcher = () => {
          calls += 1;
          return calls === 1
            ? Promise.resolve<unknown>(old)
            : new Promise<unknown>(() => {});
        };
        let response: SWRResponse<unknown> | undefined;
        function Holder() {
          response = useSWRAuth<unknown>(name, fetcher, {
            dedupingInterval: 0,
          });
          (seen[name] ??= []).push(response.data);
          return (
            <p data-testid={name}>
              {response.data === undefined
                ? "leer"
                : JSON.stringify(response.data)}
            </p>
          );
        }
        const ui = (
          <Page>
            <Holder />
          </Page>
        );
        const view = render(ui);
        await flush();
        // Anker: der Wert gilt in seiner Generation.
        expect(screen.getByTestId(name)).toHaveTextContent("alt");
        _setSWRCleanupForTests(cleanup);
        act(() => operation(() => view.rerender(cloneElement(ui))));
        await flush();
        seen = {};
        view.rerender(<Page />);
        view.rerender(cloneElement(ui));
        await flush();
        const updater = vi.fn((current: unknown) => current);
        let result: unknown = "offen";
        await act(async () => {
          result = await response!.mutate(updater, { revalidate: false });
        });
        await flush();

        expect(updater).not.toHaveBeenCalledWith(old);
        expect(result).not.toBe(old);
        expect(seen[name] ?? []).not.toContain(old);
        expect(screen.queryByText(/alt/)).toBeNull();
      },
    );
  });
});
