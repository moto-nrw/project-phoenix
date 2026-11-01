import { it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useSSE } from "./use-sse";
import {
  bindSessionAccount,
  SESSION_ACCOUNT_HEADER,
} from "~/lib/session-account";
import { installSessionTenantMismatchObserver } from "~/lib/session-tenant-mismatch-observer";
import { onSessionTenantMismatch } from "~/lib/api-error";
import type { SSEEvent } from "~/lib/sse-types";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));
vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
  getSession: vi.fn(),
}));

const fetchMock = vi.fn<typeof fetch>();
let restoreObserver: () => void;
let streams: ReadableStreamDefaultController<Uint8Array>[];
const onMessage = vi.fn();
const encoder = new TextEncoder();
function session(id: string) {
  return { data: { user: { id, token: "access" } }, status: "authenticated" };
}
function streamResponse() {
  return new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        streams.push(controller);
      },
    }),
    { headers: { "Content-Type": "text/event-stream" } },
  );
}
function sentAccount(index = 0) {
  return new Headers(fetchMock.mock.calls[index]?.[1]?.headers).get(
    SESSION_ACCOUNT_HEADER,
  );
}

beforeEach(() => {
  streams = [];
  onMessage.mockReset();
  fetchMock.mockReset().mockImplementation(async () => streamResponse());
  vi.stubGlobal("fetch", fetchMock);
  restoreObserver = installSessionTenantMismatchObserver();
  bindSessionAccount("A");
  mockUseSession.mockReturnValue(session("A"));
});
afterEach(() => {
  restoreObserver();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  bindSessionAccount(null);
});

it.each([
  "/api/sse/events",
  "/api/parent/sse/events",
  "/api/school/sse/events",
])(
  "%s binds the actual stream request and delivers named events",
  async (endpoint) => {
    const { result, unmount } = renderHook(() =>
      useSSE(endpoint, { onMessage }),
    );
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sentAccount()).toBe("A");
    // Keep the source-derived guard: every consumer branch must be reachable
    // through a NAMED frame, including future event types.
    const { readFile } = await import("node:fs/promises");
    const { resolve } = await import("node:path");
    const source = await readFile(
      resolve(process.cwd(), "src/lib/hooks/use-global-sse.ts"),
      "utf8",
    );
    const handled = [...source.matchAll(/case "([a-z_]+)":/g)].map(
      (match) => match[1]!,
    );
    expect(handled.length).toBeGreaterThan(10);
    const types = [
      ...new Set([...handled, "parent_child_updated", "class_day_changed"]),
    ];
    act(() =>
      streams[0]!.enqueue(
        encoder.encode(
          types
            .map(
              (type) =>
                `event: ${type}\ndata: ${JSON.stringify({ type, data: {} })}\n\n`,
            )
            .join(""),
        ),
      ),
    );
    await waitFor(() => expect(onMessage).toHaveBeenCalledTimes(types.length));
    expect(
      onMessage.mock.calls.map(([event]) => (event as SSEEvent).type),
    ).toEqual(types);
    const signal = fetchMock.mock.calls[0]![1]!.signal!;
    unmount();
    expect(signal.aborted).toBe(true);
  },
);

it.each([
  "identity.session_account_mismatch",
  "identity.session_tenant_mismatch",
])(
  "reports the stream's own %s refusal without a probe or retries",
  async (code) => {
    const refusal = vi.fn();
    const unsubscribe = onSessionTenantMismatch(refusal);
    fetchMock.mockResolvedValue(Response.json({ code }, { status: 409 }));
    const { result, rerender } = renderHook(() =>
      useSSE("/api/sse/events", { onMessage, reconnectInterval: 1 }),
    );
    await waitFor(() => expect(refusal).toHaveBeenCalledWith(code));
    expect(result.current.isConnected).toBe(false);
    act(() => {
      window.dispatchEvent(new Event("online"));
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    // A confirmed new provider account replaces the refused connection.
    bindSessionAccount("B");
    mockUseSession.mockReturnValue(session("B"));
    fetchMock.mockImplementation(async () => streamResponse());
    rerender();
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(sentAccount(1)).toBe("B");
    unsubscribe();
  },
);

it.each([401, 503, "offline"])(
  "retries ordinary %s errors, still bound to A",
  async (failure) => {
    const refusal = vi.fn();
    const unsubscribe = onSessionTenantMismatch(refusal);
    if (typeof failure === "number")
      fetchMock.mockResolvedValueOnce(new Response(null, { status: failure }));
    else fetchMock.mockRejectedValueOnce(new TypeError("offline"));
    const { result } = renderHook(() =>
      useSSE("/api/sse/events", { onMessage, reconnectInterval: 1 }),
    );
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect([sentAccount(), sentAccount(1)]).toEqual(["A", "A"]);
    expect(refusal).not.toHaveBeenCalled();
    unsubscribe();
  },
);

it("binds another reconnect and aborts the old stream on an account change", async () => {
  const { result, rerender } = renderHook(() =>
    useSSE("/api/sse/events", { onMessage, reconnectInterval: 1 }),
  );
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  act(() => streams[0]!.close());
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  expect(sentAccount(1)).toBe("A");
  const oldSignal = fetchMock.mock.calls[1]![1]!.signal!;
  bindSessionAccount("B");
  mockUseSession.mockReturnValue(session("B"));
  rerender();
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
  expect(oldSignal.aborted).toBe(true);
  expect(sentAccount(2)).toBe("B");
});

it("parses split UTF-8, CRLF, multiline data and ignores heartbeats", async () => {
  const { result } = renderHook(() => useSSE("/api/sse/events", { onMessage }));
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  const bytes = encoder.encode(
    ': heartbeat\r\nevent: student_updated\r\ndata: {"type":"student_updated",\r\ndata: "data":{"name":"Müller"}}\r\n\r\n',
  );
  // Split every byte: includes CR/LF boundaries and the two-byte ü.
  act(() => {
    for (const byte of bytes) streams[0]!.enqueue(new Uint8Array([byte]));
  });
  await waitFor(() =>
    expect(onMessage).toHaveBeenCalledWith({
      type: "student_updated",
      data: { name: "Müller" },
    }),
  );
  expect(onMessage).toHaveBeenCalledTimes(1);
});

it("resumes a refused stream when a lookup confirms the same account again", async () => {
  fetchMock.mockResolvedValueOnce(
    Response.json(
      { code: "identity.session_account_mismatch" },
      { status: 409 },
    ),
  );
  const { result, rerender } = renderHook(() =>
    useSSE("/api/sse/events", { onMessage }),
  );
  await waitFor(() => expect(result.current.error).toBeTruthy());
  mockUseSession.mockReturnValue(session("A"));
  rerender();
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  expect([sentAccount(), sentAccount(1)]).toEqual(["A", "A"]);
});

it("disabling aborts a pending request and ignores its late result", async () => {
  let resolve: (response: Response) => void = () => {};
  fetchMock.mockImplementationOnce(
    () =>
      new Promise<Response>((done) => {
        resolve = done;
      }),
  );
  const { result, rerender } = renderHook(
    ({ enabled }) => useSSE("/api/sse/events", { enabled, onMessage }),
    { initialProps: { enabled: true } },
  );
  const signal = fetchMock.mock.calls[0]![1]!.signal!;
  rerender({ enabled: false });
  expect(signal.aborted).toBe(true);
  await act(async () => resolve(streamResponse()));
  expect(result.current.status).toBe("idle");
  expect(onMessage).not.toHaveBeenCalled();
});

// Native EventSource support is no longer a prerequisite; fetch owns the stream.
it("streams even when native EventSource is unavailable", async () => {
  vi.stubGlobal("EventSource", undefined);
  const { result } = renderHook(() => useSSE("/api/sse/events", { onMessage }));
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  expect(sentAccount()).toBe("A");
});

it("keeps an open stream when the provider confirms the same account (#5017)", async () => {
  const { result, rerender } = renderHook(() =>
    useSSE("/api/sse/events", { onMessage }),
  );
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  const signal = fetchMock.mock.calls[0]![1]!.signal!;
  // Every session poll hands a new object for the same account.
  mockUseSession.mockReturnValue(session("A"));
  rerender();
  mockUseSession.mockReturnValue(session("A"));
  rerender();
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(signal.aborted).toBe(false);
  expect(result.current.isConnected).toBe(true);
});

it("retries a stream out of retries when the provider answers again", async () => {
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 503 }));
  const { result, rerender } = renderHook(() =>
    useSSE("/api/sse/events", { onMessage, maxReconnectAttempts: 0 }),
  );
  await waitFor(() => expect(result.current.error).toBeTruthy());
  expect(result.current.status).toBe("failed");
  expect(fetchMock).toHaveBeenCalledTimes(1);
  mockUseSession.mockReturnValue(session("A"));
  rerender();
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  expect(fetchMock).toHaveBeenCalledTimes(2);
  expect(sentAccount(1)).toBe("A");
});

it.each([
  [
    "permissions granted",
    { permissions: [] },
    { permissions: ["users:read", "parent_messages:read"] },
  ],
  [
    "permissions revoked",
    { permissions: ["users:read", "parent_messages:read"] },
    { permissions: [] },
  ],
  ["admin granted", { isAdmin: false }, { isAdmin: true }],
  ["admin revoked", { isAdmin: true }, { isAdmin: false }],
  ["roles changed", { roles: ["user"] }, { roles: ["admin"] }],
])(
  "replaces the stream when the same account's %s",
  async (_name, before, after) => {
    const initial = session("A");
    mockUseSession.mockReturnValue({
      ...initial,
      data: { user: { ...initial.data.user, ...before } },
    });
    const { result, rerender } = renderHook(() =>
      useSSE("/api/sse/events", { onMessage }),
    );
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    const signal = fetchMock.mock.calls[0]![1]!.signal!;

    mockUseSession.mockReturnValue({
      ...initial,
      data: { user: { ...initial.data.user, ...after } },
    });
    rerender();
    expect(signal.aborted).toBe(true);
    expect(result.current.isConnected).toBe(false);
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(sentAccount(1)).toBe("A");
  },
);

it("keeps the stream for reordered permissions and roles and profile changes", async () => {
  const initial = session("A");
  mockUseSession.mockReturnValue({
    ...initial,
    data: {
      user: {
        ...initial.data.user,
        roles: ["user", "teacher"],
        permissions: ["users:read", "parent_messages:read"],
        isAdmin: false,
      },
    },
  });
  const { result, rerender } = renderHook(() =>
    useSSE("/api/sse/events", { onMessage }),
  );
  await waitFor(() => expect(result.current.isConnected).toBe(true));
  const signal = fetchMock.mock.calls[0]![1]!.signal!;

  mockUseSession.mockReturnValue({
    ...initial,
    data: {
      user: {
        ...initial.data.user,
        roles: ["teacher", "user"],
        permissions: ["parent_messages:read", "users:read"],
        isAdmin: false,
        name: "Updated name",
      },
    },
  });
  rerender();
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(signal.aborted).toBe(false);
  expect(result.current.isConnected).toBe(true);
});
