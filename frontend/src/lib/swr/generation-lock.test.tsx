import { act, render, renderHook, screen } from "@testing-library/react";
import type { Session } from "next-auth";
import {
  cloneElement,
  isValidElement,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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
import { _resetSWRFallbacksForTests, preloadSWR } from "./generation";
import {
  useDiscardRefusedEntity,
  useSWRAuth,
  useTenantMutateMatching,
} from "./hooks";
import {
  _setSWRCleanupForTests,
  resetSWRScope,
  resumeSWRSession,
} from "./scope";
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
  options?: SWRConfiguration<string>;
}>) {
  const response = useSWRAuth<string>(name, fetcher, options);
  responses[name] = response;
  (seen[name] ??= []).push(response.data);
  return <p data-testid={name}>{response.data ?? "leer"}</p>;
}

let tools: {
  discard: ReturnType<typeof useDiscardRefusedEntity>;
  matching: ReturnType<typeof useTenantMutateMatching>;
};
function Tools() {
  tools = {
    discard: useDiscardRefusedEntity(),
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

    it("shows, logs and retries no late error", async () => {
      const answer = deferred<string>();
      let calls = 0;
      const fetcher = () => {
        calls += 1;
        return calls === 1 ? answer.promise : Promise.resolve("neu");
      };
      const onError = vi.fn();
      const onErrorRetry = vi.fn();
      const ui = (
        <Page>
          <Reader
            name="student-7-late-error"
            fetcher={fetcher}
            options={{ onError, onErrorRetry }}
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
      const ui = (
        <Page>
          <Reader
            name="student-7-retry"
            fetcher={fetcher}
            options={{ onErrorRetry: retry, dedupingInterval: 0 }}
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
  });
});
