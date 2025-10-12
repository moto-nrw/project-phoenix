import { useEffect, useState, useRef, useCallback } from "react";
import type { SSEEvent, SSEHookOptions, SSEHookState } from "../sse-types";

/**
 * React hook for Server-Sent Events (SSE) with auto-reconnection
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Connection state tracking
 * - Cleanup on unmount
 * - Event parsing and error handling
 *
 * @param endpoint - The SSE endpoint URL (e.g., '/api/sse/events')
 * @param options - Hook configuration options
 * @returns Connection state (isConnected, error, reconnectAttempts)
 */
export function useSSE(
  endpoint: string,
  options: SSEHookOptions = {}
): SSEHookState {
  const {
    onMessage,
    onError,
    reconnectInterval = 1000, // Start with 1 second
    maxReconnectAttempts = 5,
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const mountedRef = useRef(true);
  const reconnectAttemptsRef = useRef(0); // Track live count to avoid stale closure

  // Stable onMessage callback
  const stableOnMessage = useCallback(
    (event: SSEEvent) => {
      if (mountedRef.current) {
        onMessage?.(event);
      }
    },
    [onMessage]
  );

  // Stable onError callback
  const stableOnError = useCallback(
    (err: Event) => {
      if (mountedRef.current) {
        onError?.(err);
      }
    },
    [onError]
  );

  useEffect(() => {
    // Ensure mountedRef is true when effect runs (critical for reconnection)
    mountedRef.current = true;

    // Check if EventSource is supported
    if (typeof EventSource === "undefined") {
      console.warn("EventSource not supported in this browser");
      setError("Browser does not support Server-Sent Events");
      return;
    }

    let eventSource: EventSource | null = null;

    const connect = () => {
      if (!mountedRef.current) return;

      try {
        console.log("Establishing SSE connection...");
        eventSource = new EventSource(endpoint);
        eventSourceRef.current = eventSource;

        eventSource.onopen = () => {
          if (!mountedRef.current) return;
          console.log("SSE connected");
          setIsConnected(true);
          setError(null);
          reconnectAttemptsRef.current = 0; // Reset ref
          setReconnectAttempts(0); // Reset state
        };

        // Handle default message events
        eventSource.onmessage = (event) => {
          if (!mountedRef.current) return;
          try {
            const parsed = JSON.parse(String(event.data)) as SSEEvent;
            stableOnMessage(parsed);
          } catch (err) {
            console.error("Failed to parse SSE message:", err);
          }
        };

        // Handle specific event types
        const eventTypes = [
          "student_checkin",
          "student_checkout",
          "activity_start",
          "activity_end",
          "activity_update",
        ];

        eventTypes.forEach((eventType) => {
          eventSource?.addEventListener(eventType, (event: Event) => {
            if (!mountedRef.current) return;
            try {
              const messageEvent = event as MessageEvent;
              const parsed = JSON.parse(String(messageEvent.data)) as SSEEvent;
              stableOnMessage(parsed);
            } catch (err) {
              console.error(`Failed to parse ${eventType} event:`, err);
            }
          });
        });

        eventSource.onerror = (err) => {
          if (!mountedRef.current) return;

          console.error("SSE error:", err);
          setIsConnected(false);
          stableOnError(err);

          // Close the failed connection
          eventSource?.close();
          eventSourceRef.current = null;

          // Attempt reconnection with exponential backoff
          // Read from ref to get live count (not stale closure value)
          const currentAttempts = reconnectAttemptsRef.current;

          if (currentAttempts < maxReconnectAttempts) {
            const delay = reconnectInterval * Math.pow(2, currentAttempts);
            console.log(
              `SSE reconnecting in ${delay}ms... (attempt ${currentAttempts + 1}/${maxReconnectAttempts})`
            );

            // Update both ref (for next closure) and state (for UI)
            reconnectAttemptsRef.current = currentAttempts + 1;
            setReconnectAttempts(currentAttempts + 1);

            if (reconnectTimeoutRef.current) {
              clearTimeout(reconnectTimeoutRef.current);
            }

            reconnectTimeoutRef.current = setTimeout(() => {
              if (mountedRef.current) {
                connect();
              }
            }, delay);
          } else {
            setError("Max reconnection attempts reached");
            console.error("SSE: Max reconnection attempts reached");
          }
        };
      } catch (err) {
        console.error("Failed to create EventSource:", err);
        setError("Failed to establish SSE connection");
      }
    };

    // Initial connection
    connect();

    // Cleanup on unmount
    return () => {
      mountedRef.current = false;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      if (eventSourceRef.current) {
        console.log("Closing SSE connection");
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reconnectAttempts intentionally excluded to prevent cleanup/reconnect loop
  }, [
    endpoint,
    stableOnMessage,
    stableOnError,
    reconnectInterval,
    maxReconnectAttempts,
  ]);

  return {
    isConnected,
    error,
    reconnectAttempts,
  };
}
