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
    // Simulate async connection
    setTimeout(() => {
      this.readyState = this.OPEN;
    }, 0);
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

      // Trigger connection success
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
        expect(result.current.status).toBe("connected");
      });
    });

    it("should call onMessage when event received", async () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });

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
      });
    });

    it("should handle typed events (student_checkin, etc.)", async () => {
      const onMessage = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

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
      });
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
      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      }, { timeout: 200 });

      // Trigger error
      mockEventSource?.triggerError();

      await waitFor(() => {
        expect(result.current.isConnected).toBe(false);
        expect(result.current.reconnectAttempts).toBe(1);
        expect(result.current.status).toBe("reconnecting");
      }, { timeout: 200 });
    });

    it("should use exponential backoff for reconnection", () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 5,
        })
      );

      // First connection
      mockEventSource?.triggerOpen();
      expect(result.current.isConnected).toBe(true);

      // First error - should schedule retry after 1000ms (1000 * 2^0)
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(1);

      // Advance to first retry
      vi.advanceTimersByTime(1000);
      expect(mockEventSource).toBeTruthy();

      // Second error - should schedule retry after 2000ms (1000 * 2^1)
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(2);

      // Advance to second retry
      vi.advanceTimersByTime(2000);
      expect(mockEventSource).toBeTruthy();

      // Third error - should schedule retry after 4000ms (1000 * 2^2)
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(3);

      vi.useRealTimers();
    });

    it("should stop retrying after max attempts", () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 2,
        })
      );

      // Initial connection
      mockEventSource?.triggerOpen();
      expect(result.current.isConnected).toBe(true);

      // First error
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(1);

      vi.advanceTimersByTime(1000);

      // Second error
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(2);

      vi.advanceTimersByTime(2000);

      // Should stop retrying
      expect(result.current.status).toBe("failed");
      expect(result.current.error).toBe("Max reconnection attempts reached");

      vi.useRealTimers();
    });

    it("should reset reconnect attempts on successful connection", () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 5,
        })
      );

      // Initial connection
      mockEventSource?.triggerOpen();
      expect(result.current.isConnected).toBe(true);

      // Trigger error
      mockEventSource?.triggerError();
      expect(result.current.reconnectAttempts).toBe(1);

      // Wait for retry
      vi.advanceTimersByTime(1000);

      // Successful reconnection
      mockEventSource?.triggerOpen();
      expect(result.current.isConnected).toBe(true);
      expect(result.current.reconnectAttempts).toBe(0);
      expect(result.current.status).toBe("connected");

      vi.useRealTimers();
    });
  });

  describe("Status Transitions", () => {
    it("should transition: idle → connected", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      expect(result.current.status).toBe("idle");

      mockEventSource?.triggerOpen();

      await waitFor(() => {
        expect(result.current.status).toBe("connected");
      });
    });

    it("should transition: connected → reconnecting", async () => {
      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          maxReconnectAttempts: 5,
        })
      );

      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.status).toBe("connected");
      });

      mockEventSource?.triggerError();

      await waitFor(() => {
        expect(result.current.status).toBe("reconnecting");
      });
    });

    it("should transition: reconnecting → connected on successful retry", () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 1000,
          maxReconnectAttempts: 3,
        })
      );

      mockEventSource?.triggerOpen();
      expect(result.current.status).toBe("connected");

      mockEventSource?.triggerError();
      expect(result.current.status).toBe("reconnecting");

      vi.advanceTimersByTime(1000);
      mockEventSource?.triggerOpen();

      expect(result.current.status).toBe("connected");

      vi.useRealTimers();
    });

    it("should transition: reconnecting → failed after max attempts", () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 100,
          maxReconnectAttempts: 2,
        })
      );

      mockEventSource?.triggerOpen();
      expect(result.current.status).toBe("connected");

      // Fail twice
      for (let i = 0; i < 2; i++) {
        mockEventSource?.triggerError();
        vi.advanceTimersByTime(100 * Math.pow(2, i));
      }

      expect(result.current.status).toBe("failed");

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

    it("should clear reconnect timeout on unmount", () => {
      vi.useFakeTimers();

      const { unmount } = renderHook(() =>
        useSSE("/api/sse/events", {
          reconnectInterval: 5000,
        })
      );

      mockEventSource?.triggerOpen();
      expect(mockEventSource?.readyState).toBe(mockEventSource?.OPEN);

      // Trigger error to start reconnection timer
      mockEventSource?.triggerError();

      // Unmount before timeout completes
      unmount();

      // Advance time - reconnection should not happen
      vi.advanceTimersByTime(5000);

      // No new EventSource should be created
      expect(mockEventSource?.readyState).toBe(mockEventSource?.CLOSED);

      vi.useRealTimers();
    });

    it("should prevent state updates after unmount", async () => {
      const { result, unmount } = renderHook(() =>
        useSSE("/api/sse/events")
      );

      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });

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
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      // Wait for connection to be established
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(mockEventSource).toBeTruthy();
      }, { timeout: 100 });

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
          expect(onMessage).toHaveBeenCalled();
        },
        { timeout: 100 }
      );

      expect(onMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "activity_start",
          active_group_id: "789",
          data: expect.objectContaining({
            activity_name: "Test Activity",
          }),
        })
      );
    });

    it("should handle all event types", async () => {
      const onMessage = vi.fn();
      renderHook(() =>
        useSSE("/api/sse/events", { onMessage })
      );

      // Wait for connection to be established
      mockEventSource?.triggerOpen();
      await waitFor(() => {
        expect(mockEventSource).toBeTruthy();
      }, { timeout: 100 });

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
          expect(onMessage).toHaveBeenCalled();
        },
        { timeout: 100 }
      );

      expect(onMessage).toHaveBeenCalledTimes(5);
    });
  });
});
