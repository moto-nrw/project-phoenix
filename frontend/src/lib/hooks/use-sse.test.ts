import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

// A minimal EventSource stand-in that records every addEventListener name so we
// can assert which NAMED SSE events the hook subscribes to. The browser's real
// EventSource only delivers a named event (e.g. `event: parent_child_updated`)
// to a listener registered for that exact name — onmessage fires for UNNAMED
// events only — so a name missing here is an event the app never receives.
class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  readyState = 0;
  onopen: ((this: EventSource, ev: Event) => unknown) | null = null;
  onmessage: ((this: EventSource, ev: MessageEvent) => unknown) | null = null;
  onerror: ((this: EventSource, ev: Event) => unknown) | null = null;
  registered: string[] = [];

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }
  addEventListener(type: string): void {
    this.registered.push(type);
  }
  close(): void {
    this.readyState = 2;
  }
}

describe("useSSE — named event registration", () => {
  let useSSE: typeof import("./use-sse").useSSE;

  beforeEach(async () => {
    MockEventSource.instances = [];
    vi.stubGlobal("EventSource", MockEventSource);
    ({ useSSE } = await import("./use-sse"));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("subscribes to the new #1845 named events so their handlers are reachable", () => {
    renderHook(() => useSSE("/api/sse/events", { onMessage: () => undefined }));

    const es = MockEventSource.instances.at(-1);
    expect(es).toBeDefined();
    // The two events the review flagged as unreachable: without these
    // registrations the staff review queue and the parents care view only
    // refresh on focus/manual reload.
    expect(es!.registered).toContain("change_requests_changed");
    expect(es!.registered).toContain("parent_child_updated");
    // Sanity: pre-existing named events stay registered.
    expect(es!.registered).toContain("parent_message");
    expect(es!.registered).toContain("student_updated");
  });

  it("registers every event type useGlobalSSE handles, so no branch is dead", async () => {
    // The registration list and the handler switch drifted apart twice already
    // (staffing_deviation_changed and student_companions_changed both shipped
    // with a handler no event could ever reach — the backend sends every event
    // NAMED, so an unregistered name is silently dropped and the only symptom
    // is a view that never refreshes). Deriving the expected set from the
    // handler source makes the next omission fail here instead of in
    // production.
    const { readFile } = await import("node:fs/promises");
    const path = await import("node:path");
    // Resolved from the vitest root (the frontend package dir), not
    // import.meta.url: under the jsdom environment that URL is not a file: URL,
    // so readFile rejects it.
    const source = await readFile(
      path.resolve(process.cwd(), "src/lib/hooks/use-global-sse.ts"),
      "utf8",
    );
    // The file has exactly one switch, and it switches on event.type.
    const handled = [...source.matchAll(/case "([a-z_]+)":/g)]
      .map((m) => m[1])
      .filter((type): type is string => type !== undefined);
    expect(handled.length).toBeGreaterThan(10);

    renderHook(() => useSSE("/api/sse/events", { onMessage: () => undefined }));
    const registered = MockEventSource.instances.at(-1)!.registered;

    expect(handled.filter((type) => !registered.includes(type))).toEqual([]);
  });
});
