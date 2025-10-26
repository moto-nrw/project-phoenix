import { renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { useSSE } from "../use-sse";
import type { SSEEvent } from "../../sse-types";

// Mock EventSource
class MockEventSource {
  public url: string;
  public onopen: ((event: Event) => void) | null = null;
  public onmessage: ((event: MessageEvent) => void) | null = null;
  public onerror: ((event: Event) => void) | null = null;
  public readyState = 0;
  public CONNECTING = 0;
  public OPEN = 1;
  public CLOSED = 2;
  private eventListeners = new Map<string, ((event: Event) => void)[]>();

  constructor(url: string) {
    this.url = url;
    this.readyState = this.CONNECTING;
    // Tests will explicitly call triggerOpen() when ready
  }

  addEventListener(type: string, listener: (event: Event) => void): void {
    if (!this.eventListeners.has(type)) {
      this.eventListeners.set(type, []);
    }
    this.eventListeners.get(type)!.push(listener);
  }

  removeEventListener(type: string, listener: (event: Event) => void): void {
    const listeners = this.eventListeners.get(type);
    if (listeners) {
      const index = listeners.indexOf(listener);
      if (index > -1) {
        listeners.splice(index, 1);
      }
    }
  }

  close(): void {
    this.readyState = this.CLOSED;
  }

  // Test helper methods
  triggerOpen(): void {
    this.readyState = this.OPEN;
    if (this.onopen) {
      this.onopen(new Event("open"));
    }
  }

  triggerMessage(data: SSEEvent, eventType?: string): void {
    const event = new MessageEvent(eventType ?? "message", {
      data: JSON.stringify(data),
    });

    if (eventType && this.eventListeners.has(eventType)) {
      this.eventListeners.get(eventType)!.forEach((listener) => {
        listener(event);
      });
    } else if (this.onmessage) {
      this.onmessage(event);
    }
  }

  triggerError(): void {
    this.readyState = this.CLOSED;
    if (this.onerror) {
      this.onerror(new Event("error"));
    }
  }
}

describe("useSSE Hook", () => {
  let mockEventSource: MockEventSource | null = null;
  let eventSourceInstances: MockEventSource[] = [];

  // Helper: Wait for EventSource to be created and handlers attached
  const waitForEventSource = async (timeout = 500) => {
    await waitFor(() => {
      expect(mockEventSource).toBeTruthy();
      expect(mockEventSource?.onopen).toBeTruthy();
    }, { timeout });
  };

  // Helper: Get the latest EventSource instance
  const getLatestEventSource = () => eventSourceInstances[eventSourceInstances.length - 1];
  const requireLatestEventSource = () => {
    const instance = getLatestEventSource();
    if (!instance) {
      throw new Error("EventSource instance not initialized");
    }
    return instance;
  };

  beforeEach(() => {
    eventSourceInstances = [];

    // Replace global EventSource with our mock
    vi.stubGlobal("EventSource", vi.fn((url: string) => {
      const instance = new MockEventSource(url);
      mockEventSource = instance;
      eventSourceInstances.push(instance);
      return instance;
    }));

    // Mock console methods to reduce test noise
    vi.spyOn(console, "log").mockImplementation(() => undefined);
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    mockEventSource = null;
    eventSourceInstances = [];
  });

  describe("Initial Connection", () => {
    it("should start in idle state", () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      expect(result.current.isConnected).toBe(false);
      expect(result.current.reconnectAttempts).toBe(0);
      expect(result.current.status).toBe("idle");
      expect(result.current.error).toBe(null);
    });

    it("should establish connection on mount", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      // Wait for EventSource to be created and handlers attached
      await waitForEventSource();

      // Trigger connection success
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
        expect(result.current.status).toBe("connected");
      }, { timeout: 500 });
    });

    it("should call onMessage when event received", async () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      await waitForEventSource();
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      const testEvent: SSEEvent = {
        type: "student_checkin",
        active_group_id: "123",
        data: {
          student_id: "456",
          student_name: "Test Student",
        },
        timestamp: new Date().toISOString(),
      };

      mockEventSource?.triggerMessage(testEvent);

      await waitFor(() => {
        expect(onMessage).toHaveBeenCalledWith(testEvent);
      }, { timeout: 500 });
    });

    it("should handle typed events (student_checkin, etc.)", async () => {
      const onMessage = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      await waitForEventSource();
      mockEventSource?.triggerOpen();

      const testEvent: SSEEvent = {
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "456" },
        timestamp: new Date().toISOString(),
      };

      mockEventSource?.triggerMessage(testEvent, "student_checkin");

      await waitFor(() => {
        expect(onMessage).toHaveBeenCalledWith(testEvent);
      }, { timeout: 500 });
    });
  });

  describe("Reconnection Logic", () => {
    it("should attempt reconnection on error", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 3,
        })
      );

      // Initial connection
      await waitForEventSource();
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      // Trigger error
      mockEventSource?.triggerError();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(false);
        expect(result.current.reconnectAttempts).toBe(1);
        expect(result.current.status).toBe("reconnecting");
      }, { timeout: 500 });
    });

    it("should use exponential backoff for reconnection", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 10, // Use short delays for fast tests
          maxReconnectAttempts: 5,
        })
      );

      // First connection
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // First error - should schedule retry after 10ms (10 * 2^0)
      firstInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Wait for first retry - new EventSource created
      await waitFor(() => expect(eventSourceInstances.length).toBe(2), { timeout: 500 });
      const secondInstance = requireLatestEventSource();

      // Second error - should schedule retry after 20ms (10 * 2^1)
      secondInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      // Wait for second retry
      await waitFor(() => expect(eventSourceInstances.length).toBe(3), { timeout: 500 });

      // Third error - should schedule retry after 40ms (10 * 2^2)
      const thirdInstance = requireLatestEventSource();
      thirdInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(3), { timeout: 500 });
    });

    it("should stop retrying after max attempts", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 10, // Use short delays for fast tests
          maxReconnectAttempts: 2,
        })
      );

      // Initial connection
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // First error
      firstInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Wait for first retry
      await waitFor(() => expect(eventSourceInstances.length).toBe(2), { timeout: 500 });
      const secondInstance = requireLatestEventSource();

      // Second error - this is attempt 2, should hit max
      secondInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      // Should stop retrying and transition to failed
      await waitFor(() => {
        expect(result.current.status).toBe("failed");
      }, { timeout: 1000 });
    });

    it("should reset reconnect attempts on successful connection", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 10, // Use short delays for fast tests
          maxReconnectAttempts: 5,
        })
      );

      // Initial connection
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // Trigger error
      firstInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Wait for retry - new EventSource created
      await waitFor(() => expect(eventSourceInstances.length).toBe(2), { timeout: 500 });
      const secondInstance = requireLatestEventSource();

      // Successful reconnection
      secondInstance.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
        expect(result.current.reconnectAttempts).toBe(0);
        expect(result.current.status).toBe("connected");
      }, { timeout: 500 });
    });
  });

  describe("Status Transitions", () => {
    it("should transition: idle → connected", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      expect(result.current.status).toBe("idle");

      await waitForEventSource();
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.status).toBe("connected");
      }, { timeout: 500 });
    });

    it("should transition: connected → reconnecting", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          maxReconnectAttempts: 5,
        })
      );

      await waitForEventSource();
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.status).toBe("connected");
      }, { timeout: 500 });

      mockEventSource?.triggerError();

      await waitFor(() => {
        expect(result.current.status).toBe("reconnecting");
      }, { timeout: 500 });
    });

    it("should transition: reconnecting → connected on successful retry", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 10, // Use short delays for fast tests
          maxReconnectAttempts: 3,
        })
      );

      // Initial connection
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });

      firstInstance.triggerError();
      await waitFor(() => expect(result.current.status).toBe("reconnecting"), { timeout: 500 });

      await waitFor(() => expect(eventSourceInstances.length).toBe(2), { timeout: 500 });
      const secondInstance = requireLatestEventSource();
      secondInstance.triggerOpen();

      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });
    });

    it("should transition: reconnecting → failed after max attempts", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 10, // Use short delays for fast tests
          maxReconnectAttempts: 2,
        })
      );

      // Initial connection
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });

      // First error
      firstInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Wait for first retry
      await waitFor(() => expect(eventSourceInstances.length).toBe(2), { timeout: 500 });
      const secondInstance = requireLatestEventSource();

      // Second error - hits max
      secondInstance.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      // Should transition to failed
      await waitFor(() => expect(result.current.status).toBe("failed"), { timeout: 500 });
    });
  });

  describe("Cleanup on Unmount", () => {
    it("should close EventSource on unmount", async () => {
      const { unmount } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      await waitForEventSource();
      const instance = requireLatestEventSource();
      instance.triggerOpen();

      const closeSpy = vi.spyOn(instance, "close");

      unmount();

      expect(closeSpy).toHaveBeenCalled();
    });

    it("should clear reconnect timeout on unmount", async () => {
      const { unmount } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 5000,
        })
      );

      // Initial connection (with real timers)
      await waitForEventSource();
      const firstInstance = requireLatestEventSource();
      firstInstance.triggerOpen();
      await waitFor(() => expect(firstInstance.readyState).toBe(firstInstance.OPEN), { timeout: 500 });

      // Trigger error to start reconnection timer
      firstInstance.triggerError();
      await waitFor(() => expect(firstInstance.readyState).toBe(firstInstance.CLOSED), { timeout: 500 });

      const instanceCountBeforeUnmount = eventSourceInstances.length;

      // Unmount before timeout completes
      unmount();

      // Switch to fake timers and flush any pending timers
      vi.useFakeTimers();
      await vi.runAllTimersAsync();
      vi.useRealTimers();

      // Old instance should remain closed, no new instance created
      expect(firstInstance.readyState).toBe(firstInstance.CLOSED);
      expect(eventSourceInstances.length).toBe(instanceCountBeforeUnmount); // No new instance
    });

    it("should prevent state updates after unmount", async () => {
      const { result, unmount } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      await waitForEventSource();
      const instance = requireLatestEventSource();
      instance.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      unmount();

      // Try to trigger events after unmount (should not cause errors)
      expect(() => {
        instance.triggerMessage({
          type: "student_checkin",
          active_group_id: "123",
          data: {},
          timestamp: new Date().toISOString(),
        });
      }).not.toThrow();
    });
  });

  describe("Error Handling", () => {
    it("should call onError callback when error occurs", async () => {
      const onError = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onError })
      );

      await waitForEventSource();
      mockEventSource?.triggerOpen();
      mockEventSource?.triggerError();

      await waitFor(() => expect(onError).toHaveBeenCalled(), { timeout: 500 });
    });

    it("should handle parse errors gracefully", async () => {
      const onMessage = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      await waitForEventSource();
      mockEventSource?.triggerOpen();

      // Send invalid JSON
      const invalidEvent = new MessageEvent("message", {
        data: "invalid json{{{",
      });

      mockEventSource!.onmessage?.(invalidEvent);

      // Wait a bit to ensure no message was processed
      await new Promise((resolve) => setTimeout(resolve, 100));

      // Should not crash, onMessage should not be called
      expect(onMessage).not.toHaveBeenCalled();
    });

    it("should warn when EventSource is not supported", () => {
      vi.stubGlobal("EventSource", undefined);

      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      expect(result.current.error).toBe(
        "Browser does not support Server-Sent Events"
      );
    });
  });

  describe("Event Parsing", () => {
    it("should parse JSON event data correctly", async () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      // Wait for connection to be established
      await waitForEventSource();
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      const testEvent: SSEEvent = {
        type: "activity_start",
        active_group_id: "789",
        data: {
          activity_name: "Test Activity",
          room_id: "5",
          supervisor_ids: ["1", "2"],
        },
        timestamp: new Date().toISOString(),
      };

      mockEventSource?.triggerMessage(testEvent);

      await waitFor(() => {
        expect(onMessage).toHaveBeenCalled();
      }, { timeout: 500 });

      const lastCall = onMessage.mock.calls.at(-1);
      expect(lastCall).toBeDefined();

      const [payload] = lastCall as [SSEEvent];
      expect(payload.type).toBe("activity_start");
      expect(payload.active_group_id).toBe("789");
      expect(payload.data.activity_name).toBe("Test Activity");
    });

    it("should handle all event types", async () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      // Wait for connection to be established
      await waitForEventSource();
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      const eventTypes: Array<SSEEvent["type"]> = [
        "student_checkin",
        "student_checkout",
        "activity_start",
        "activity_end",
        "activity_update",
      ];

      for (const eventType of eventTypes) {
        mockEventSource?.triggerMessage(
          {
            type: eventType,
            active_group_id: "123",
            data: {},
            timestamp: new Date().toISOString(),
          },
          eventType
        );
      }

      await waitFor(
        () => {
          expect(onMessage).toHaveBeenCalledTimes(5);
        },
        { timeout: 500 }
      );
    });
  });
});
