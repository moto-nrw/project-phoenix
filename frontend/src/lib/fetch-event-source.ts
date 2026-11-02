import { apiErrorFromResponse, isSessionRefusalCode } from "./api-error";

/**
 * The backend writes a heartbeat comment every 30 s
 * (backend/modules/delivery/http/sse/sse_helpers.go, school_api.go). A
 * half-open connection delivers neither data nor heartbeats; after 2.5
 * heartbeat intervals of silence the stream counts as dropped (#5017), so
 * the tab leaves the five-minute live mode, reconnects and reconciles.
 */
export const SSE_IDLE_TIMEOUT_MS = 75_000;

/**
 * One fetch-backed SSE connection. Unlike native EventSource, the actual
 * stream request uses the portal fetch observer's account binding and can
 * read its refusal envelope. SessionAccountBinding installs the observer
 * before portal pages connect.
 * Retry policy belongs to useSSE, not to this transport.
 */
export class FetchEventSource extends EventTarget {
  readyState = 0;
  sessionRefused = false;
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private readonly controller = new AbortController();
  private idleTimer: ReturnType<typeof setTimeout> | null = null;
  private stalled = false;
  private reader: ReadableStreamDefaultReader<Uint8Array> | undefined;

  constructor(
    readonly url: string,
    private readonly idleTimeoutMs = SSE_IDLE_TIMEOUT_MS,
  ) {
    super();
    this.armIdleTimer();
    void this.read();
  }

  close() {
    this.readyState = 2;
    this.clearIdleTimer();
    this.controller.abort();
  }

  // Any received byte, a heartbeat comment included, proves the connection
  // alive. Silence beyond the timeout aborts the read as a dropped stream.
  private armIdleTimer() {
    this.clearIdleTimer();
    this.idleTimer = setTimeout(() => {
      this.idleTimer = null;
      this.stalled = true;
      this.controller.abort();
      // Ends a pending read even where the body ignores the abort signal.
      void this.reader?.cancel().catch(() => undefined);
    }, this.idleTimeoutMs);
  }

  private clearIdleTimer() {
    if (this.idleTimer) {
      clearTimeout(this.idleTimer);
      this.idleTimer = null;
    }
  }

  private async read() {
    const signal = this.controller.signal;
    let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
    try {
      const response = await fetch(this.url, {
        headers: { Accept: "text/event-stream" },
        cache: "no-store",
        signal,
      });
      if (signal.aborted) throw new Error("Event stream stalled");
      if (response.status !== 200) {
        this.readyState = 2;
        // apiErrorFromResponse feeds the existing session mismatch path.
        const error = await apiErrorFromResponse(
          response,
          "SSE request refused",
        );
        this.sessionRefused =
          response.status === 409 && isSessionRefusalCode(error.code);
        throw error;
      }
      if (
        response.headers.get("Content-Type")?.split(";")[0]?.trim() !==
          "text/event-stream" ||
        !response.body
      ) {
        this.readyState = 2;
        throw new Error("Expected an event stream");
      }
      reader = response.body.getReader();
      this.reader = reader;
      this.readyState = 1;
      this.onopen?.();
      const decoder = new TextDecoder();
      let buffer = "";
      let eventType = "message";
      let data: string[] = [];
      // SSE accepts CR, LF and CRLF line endings, even across byte chunks.
      let skipLF = false;
      const line = (value: string) => {
        if (!value) {
          if (data.length) {
            const event = new MessageEvent<string>(eventType, {
              data: data.join("\n"),
            });
            if (eventType === "message") this.onmessage?.(event);
            else this.dispatchEvent(event);
          }
          data = [];
          eventType = "message";
          return;
        }
        const colon = value.indexOf(":");
        const field = colon < 0 ? value : value.slice(0, colon);
        const content =
          colon < 0 ? "" : value.slice(colon + 1).replace(/^ /, "");
        if (field === "data") data.push(content);
        if (field === "event") eventType = content || "message";
        // Comments, id and retry are ignored: no replay, hook-owned backoff.
      };
      while (!signal.aborted) {
        const chunk = await reader.read();
        if (chunk.done) break;
        this.armIdleTimer();
        for (const char of decoder.decode(chunk.value, { stream: true })) {
          if (signal.aborted) break;
          if (skipLF && char === "\n") {
            skipLF = false;
            continue;
          }
          skipLF = char === "\r";
          if (char === "\r" || char === "\n") {
            line(buffer);
            buffer = "";
          } else buffer += char;
        }
      }
      // EOF is a dropped connection, including a clean backend restart.
      throw new Error("Event stream ended");
    } catch {
      // Cancellation is cleanup, not a connection failure; a stall is one.
      // useSSE owns the visible backoff/error state for transport failures.
      if (!signal.aborted || this.stalled) {
        const event = new Event("error");
        this.dispatchEvent(event);
        this.onerror?.(event);
      }
    } finally {
      this.clearIdleTimer();
      // Cancel invalid bodies and release the live stream on every exit.
      if (reader) {
        await reader.cancel().catch(() => undefined);
        reader.releaseLock();
      }
      this.controller.abort();
    }
  }
}
