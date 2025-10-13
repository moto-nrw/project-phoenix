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
  public readyState: number = 0;
  public CONNECTING = 0;
  public OPEN = 1;
  public CLOSED = 2;
  private eventListeners: Map<string, ((event: Event) => void)[]> = new Map();

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

  beforeEach(() => {
    // Replace global EventSource with our mock
    vi.stubGlobal("EventSource", vi.fn((url: string) => {
      mockEventSource = new MockEventSource(url);
      return mockEventSource;
    }));

    // Mock console methods to reduce test noise
    vi.spyOn(console, "log").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    mockEventSource = null;
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
      await waitFor(() => expect(mockEventSource).toBeTruthy(), { timeout: 100 });

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

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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
      }, { timeout: 200 });
    });

    it("should use exponential backoff for reconnection", async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 5,
        })
      );

      // First connection
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
      mockEventSource?.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // First error - should schedule retry after 1000ms (1000 * 2^0)
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Advance to first retry - new EventSource created
      await vi.advanceTimersByTimeAsync(1000);
      await waitFor(() => expect(mockEventSource).toBeTruthy(), { timeout: 500 });

      // Second error - should schedule retry after 2000ms (1000 * 2^1)
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      // Advance to second retry
      await vi.advanceTimersByTimeAsync(2000);
      await waitFor(() => expect(mockEventSource).toBeTruthy(), { timeout: 500 });

      // Third error - should schedule retry after 4000ms (1000 * 2^2)
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(3), { timeout: 500 });

      vi.useRealTimers();
    });

    it("should stop retrying after max attempts", async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 2,
        })
      );

      // Initial connection
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
      mockEventSource?.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // First error
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      await vi.advanceTimersByTimeAsync(1000);

      // Second error
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      await vi.advanceTimersByTimeAsync(2000);

      // Should stop retrying
      await waitFor(() => {
        expect(result.current.status).toBe("failed");
        expect(result.current.error).toBe("Max reconnection attempts reached");
      }, { timeout: 500 });

      vi.useRealTimers();
    });

    it("should reset reconnect attempts on successful connection", async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 5,
        })
      );

      // Initial connection
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
      mockEventSource?.triggerOpen();
      await waitFor(() => expect(result.current.isConnected).toBe(true), { timeout: 500 });

      // Trigger error
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Wait for retry - new EventSource created
      await vi.advanceTimersByTimeAsync(1000);

      // Successful reconnection
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
        expect(result.current.reconnectAttempts).toBe(0);
        expect(result.current.status).toBe("connected");
      }, { timeout: 500 });

      vi.useRealTimers();
    });
  });

  describe("Status Transitions", () => {
    it("should transition: idle → connected", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      expect(result.current.status).toBe("idle");

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 3,
        })
      );

      mockEventSource?.triggerOpen();
      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });

      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.status).toBe("reconnecting"), { timeout: 500 });

      await vi.advanceTimersByTimeAsync(1000);
      mockEventSource?.triggerOpen();

      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });

      vi.useRealTimers();
    });

    it("should transition: reconnecting → failed after max attempts", async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 100,
          maxReconnectAttempts: 2,
        })
      );

      mockEventSource?.triggerOpen();
      await waitFor(() => expect(result.current.status).toBe("connected"), { timeout: 500 });

      // First error
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(1), { timeout: 500 });

      // Advance to first retry
      await vi.advanceTimersByTimeAsync(100);

      // Second error
      mockEventSource?.triggerError();
      await waitFor(() => expect(result.current.reconnectAttempts).toBe(2), { timeout: 500 });

      // Advance to second retry (should hit max)
      await vi.advanceTimersByTimeAsync(200);

      await waitFor(() => expect(result.current.status).toBe("failed"), { timeout: 500 });

      vi.useRealTimers();
    });
  });

  describe("Cleanup on Unmount", () => {
    it("should close EventSource on unmount", () => {
      const { unmount } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      mockEventSource?.triggerOpen();

      const closeSpy = vi.spyOn(mockEventSource!, "close");

      unmount();

      expect(closeSpy).toHaveBeenCalled();
    });

    it("should clear reconnect timeout on unmount", async () => {
      vi.useFakeTimers();

      const { unmount } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 5000,
        })
      );

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
      mockEventSource?.triggerOpen();
      await waitFor(() => expect(mockEventSource?.readyState).toBe(mockEventSource?.OPEN), { timeout: 500 });

      const oldInstance = mockEventSource;

      // Trigger error to start reconnection timer
      mockEventSource?.triggerError();
      await waitFor(() => expect(oldInstance?.readyState).toBe(oldInstance?.CLOSED), { timeout: 500 });

      // Unmount before timeout completes
      unmount();

      // Flush any pending timers
      await vi.runAllTimersAsync();

      // Old instance should remain closed, no new instance created
      expect(oldInstance?.readyState).toBe(oldInstance?.CLOSED);
      expect(mockEventSource).toBe(oldInstance); // No new instance

      vi.useRealTimers();
    });

    it("should prevent state updates after unmount", async () => {
      const { result, unmount } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 500 });

      unmount();

      // Try to trigger events after unmount (should not cause errors)
      expect(() => {
        mockEventSource?.triggerMessage({
          type: "student_checkin",
          active_group_id: "123",
          data: {},
          timestamp: new Date().toISOString(),
        });
      }).not.toThrow();
    });
  });

  describe("Error Handling", () => {
    it("should call onError callback when error occurs", () => {
      const onError = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onError })
      );

      mockEventSource?.triggerOpen();
      mockEventSource?.triggerError();

      expect(onError).toHaveBeenCalled();
    });

    it("should handle parse errors gracefully", () => {
      const onMessage = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      mockEventSource?.triggerOpen();

      // Send invalid JSON
      const invalidEvent = new MessageEvent("message", {
        data: "invalid json{{{",
      });

      mockEventSource!.onmessage?.(invalidEvent);

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
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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

      // Wait for message to be processed
      await waitFor(
        () => {
          expect(onMessage).toHaveBeenCalledWith(
            expect.objectContaining({
              type: "activity_start",
              active_group_id: "789",
              data: expect.objectContaining({
                activity_name: "Test Activity",
              }),
            })
          );
        },
        { timeout: 500 }
      );
    });

    it("should handle all event types", async () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      // Wait for connection to be established
      await waitFor(() => expect(mockEventSource?.onopen).toBeTruthy(), { timeout: 500 });
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
