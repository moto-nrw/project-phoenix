import { useEffect, useState, useRef, useCallback, useMemo } from "react";
import type {
  SSEEvent,
  SSEHookOptions,
  SSEHookState,
  ConnectionStatus,
} from "../sse-types";

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
  options: SSEHookOptions = {},
): SSEHookState {
  const {
    onMessage,
    onError,
    reconnectInterval = 1000, // Start with 1 second
    maxReconnectAttempts = 5,
    enabled = true,
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
    [onMessage],
  );

  // Stable onError callback
  const stableOnError = useCallback(
    (err: Event) => {
      if (mountedRef.current) {
        onError?.(err);
      }
    },
    [onError],
  );

  useEffect(() => {
    if (!enabled) {
      setIsConnected(false);
      setError(null);
      setReconnectAttempts(0);
      return;
    }
    // Ensure mountedRef is true when effect runs (critical for reconnection)
    mountedRef.current = true;

    // Check if EventSource is supported
    if (typeof EventSource === "undefined") {
      console.warn("EventSource not supported in this browser");
      setError("Browser does not support Server-Sent Events");
      return;
    }

    let eventSource: EventSource | null = null;

    // Event handler for SSE messages - handles parsing and error reporting
    const handleSSEMessage = (eventType: string, event: Event) => {
      if (!mountedRef.current) return;
      try {
        const messageEvent = event as MessageEvent;
        const parsed = JSON.parse(String(messageEvent.data)) as SSEEvent;
        stableOnMessage(parsed);
      } catch (err) {
        console.error(`Failed to parse ${eventType} event:`, err);
        setError(`Ungültige Server-Event-Daten (${eventType})`);
      }
    };

    const connect = () => {
      if (!mountedRef.current) return;

      // Reconnection callback
      const attemptReconnect = () => {
        if (mountedRef.current) {
          connect();
        }
      };

      try {
        eventSource = new EventSource(endpoint);
        eventSourceRef.current = eventSource;

        eventSource.onopen = () => {
          if (!mountedRef.current) return;
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
            setError("Ungültige Server-Event-Daten empfangen");
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

        for (const eventType of eventTypes) {
          eventSource.addEventListener(eventType, (event) =>
            handleSSEMessage(eventType, event),
          );
        }

        eventSource.onerror = (err) => {
          if (!mountedRef.current) return;

          // Log more detailed error information
          const errorDetails = {
            readyState: eventSource?.readyState,
            url: endpoint,
            type: err.type,
            target: err.target,
          };
          console.error("SSE connection error:", errorDetails);

          setIsConnected(false);
          // classify likely causes
          if (typeof navigator !== "undefined" && !navigator.onLine) {
            setError("Netzwerkverbindung unterbrochen");
          } else if (eventSource?.readyState === 2) {
            setError("SSE-Verbindung vom Server geschlossen");
          } else {
            setError("SSE-Verbindungsfehler");
          }
          stableOnError(err);

          // Close the failed connection
          eventSource?.close();
          eventSourceRef.current = null;

          // Attempt reconnection with exponential backoff
          // Read from ref to get live count (not stale closure value)
          const currentAttempts = reconnectAttemptsRef.current;

          if (currentAttempts < maxReconnectAttempts) {
            const delay = reconnectInterval * Math.pow(2, currentAttempts);

            // Update both ref (for next closure) and state (for UI)
            reconnectAttemptsRef.current = currentAttempts + 1;
            setReconnectAttempts(currentAttempts + 1);

            if (reconnectTimeoutRef.current) {
              clearTimeout(reconnectTimeoutRef.current);
            }

            reconnectTimeoutRef.current = setTimeout(attemptReconnect, delay);
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

    // Reconnect when device wakes from sleep or network restores
    const handleWakeReconnect = () => {
      if (!mountedRef.current) return;
      // Skip if already connected
      if (eventSourceRef.current?.readyState === 1) {
        return;
      }

      // Close dead connection if it exists
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }

      // Clear any pending reconnect timeout
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      // Reset attempts so we get fresh retries
      reconnectAttemptsRef.current = 0;
      setReconnectAttempts(0);

      connect();
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        handleWakeReconnect();
      }
    };

    const handleOnline = () => {
      handleWakeReconnect();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    globalThis.addEventListener("online", handleOnline);

    // Cleanup on unmount
    return () => {
      mountedRef.current = false;

      document.removeEventListener("visibilitychange", handleVisibilityChange);
      globalThis.removeEventListener("online", handleOnline);

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [
    endpoint,
    stableOnMessage,
    stableOnError,
    reconnectInterval,
    maxReconnectAttempts,
    enabled,
  ]);

  // Compute connection status based on state
  const status = useMemo<ConnectionStatus>(() => {
    if (isConnected) return "connected";
    if (reconnectAttempts >= maxReconnectAttempts) return "failed";
    if (reconnectAttempts > 0) return "reconnecting";
    return "idle";
  }, [isConnected, reconnectAttempts, maxReconnectAttempts]);

  return {
    isConnected,
    error,
    reconnectAttempts,
    status,
  };
}
