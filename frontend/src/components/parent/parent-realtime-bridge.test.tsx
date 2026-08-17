import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ParentRealtimeBridge } from "./parent-realtime-bridge";
import type { SSEHookOptions } from "~/lib/sse-types";

const useSSE = vi.hoisted(() => vi.fn());

vi.mock("~/lib/hooks/use-sse", () => ({ useSSE }));

describe("ParentRealtimeBridge", () => {
  it("forwards guardian-scoped notifications to the shared notification bridge", () => {
    render(<ParentRealtimeBridge />);

    expect(useSSE).toHaveBeenCalledWith(
      "/api/parent/sse/events",
      expect.anything(),
    );
    const options = useSSE.mock.calls[0]?.[1] as SSEHookOptions | undefined;
    const listener = vi.fn();
    window.addEventListener("phoenix:notification", listener);

    try {
      options?.onMessage?.({
        type: "notification",
        active_group_id: "",
        data: {
          title: "Abholzeit geändert",
          body: "Die neue Abholzeit ist verfügbar.",
          deep_link: "/children/123",
          priority: "high",
          notification_type: "pickup_changed",
          notification_data: { child_id: "123" },
        },
        timestamp: new Date().toISOString(),
      });

      expect(listener).toHaveBeenCalledOnce();
      const dispatchedEvent = listener.mock.calls[0]?.[0] as CustomEvent;
      expect(dispatchedEvent.detail).toEqual({
        title: "Abholzeit geändert",
        body: "Die neue Abholzeit ist verfügbar.",
        deepLink: "/children/123",
        priority: "high",
        notificationType: "pickup_changed",
        data: { child_id: "123" },
      });
    } finally {
      window.removeEventListener("phoenix:notification", listener);
    }
  });

  it("refreshes the affected child's daily status after a child update", () => {
    render(<ParentRealtimeBridge />);

    const options = useSSE.mock.calls.at(-1)?.[1] as SSEHookOptions | undefined;
    const onMessage = options?.onMessage;
    expect(onMessage).toBeTypeOf("function");
    if (!onMessage) throw new Error("SSE handler was not registered");
    const listener = vi.fn();
    window.addEventListener("parent-child-status-refresh", listener);

    try {
      onMessage({
        type: "parent_child_updated",
        active_group_id: "",
        data: { student_id: "123" },
        timestamp: new Date().toISOString(),
      });

      expect(listener).toHaveBeenCalledOnce();
      const event = listener.mock.calls[0]?.[0];
      expect(event).toBeInstanceOf(CustomEvent);
      if (!(event instanceof CustomEvent))
        throw new Error("Missing refresh event");
      expect(event.detail).toEqual({
        studentId: "123",
      });
    } finally {
      window.removeEventListener("parent-child-status-refresh", listener);
    }
  });
});
