import { apiErrorFromResponse, isSessionRefusalCode } from "./api-error";
import {
  boundSessionAccount,
  isOwnApiUrl,
  SESSION_ACCOUNT_HEADER,
} from "./session-account";

/**
 * One fetch-backed SSE connection. Unlike native EventSource, the actual
 * stream request can name the tab's account and read its refusal envelope.
 * Retry policy belongs to useSSE, not to this transport.
 */
export class FetchEventSource extends EventTarget {
  readyState = 0;
  sessionRefused = false;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private readonly controller = new AbortController();

  constructor(readonly url: string) {
    super();
    void this.read();
  }

  close() {
    this.readyState = 2;
    this.controller.abort();
  }

  private async read() {
    const signal = this.controller.signal;
    let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
    try {
      const headers = new Headers({ Accept: "text/event-stream" });
      const account = isOwnApiUrl(this.url) ? boundSessionAccount() : null;
      if (account !== null) headers.set(SESSION_ACCOUNT_HEADER, account);
      const response = await fetch(this.url, {
        headers,
        cache: "no-store",
        credentials: "same-origin",
        signal,
      });
      if (signal.aborted) return;
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
      this.readyState = 1;
      this.onopen?.(new Event("open"));
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
        for (const char of decoder.decode(chunk.value, { stream: true })) {
          if (signal.aborted) return;
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
      // Cancellation is cleanup, not a connection failure. useSSE owns the
      // visible backoff/error state for all other transport failures.
      if (!signal.aborted) {
        const event = new Event("error");
        this.dispatchEvent(event);
        this.onerror?.(event);
      }
    } finally {
      // Cancel invalid bodies and release the live stream on every exit.
      if (reader) {
        await reader.cancel().catch(() => undefined);
        reader.releaseLock();
      }
      this.controller.abort();
    }
  }
}
