import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { REDACTED_LOG_VALUE } from "./log-redaction";

vi.unmock("~/lib/logger");

const { createLogger } = await import("./logger");

describe("structured logger redaction", () => {
  let originalFetch: typeof globalThis.fetch;
  let originalWindow: typeof globalThis.window;
  let originalPathname: string;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    originalWindow = globalThis.window;
    originalPathname = globalThis.window.location.pathname;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
    originalWindow.history.replaceState(null, "", originalPathname);
    vi.unstubAllEnvs();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("redacts sensitive context before server stdout", () => {
    // @ts-expect-error -- force server detection in the happy-dom test runtime
    globalThis.window = undefined;
    const consoleLog = vi.spyOn(console, "log").mockImplementation(() => {});
    const logger = createLogger({ component: "ServerProbe", level: "debug" });

    logger.info("server_redaction_probe", {
      password: "server-password-sentinel",
      nested: {
        refresh_token: "server-token-sentinel",
        jwt: "server-jwt-sentinel",
        apiKey: "server-api-key-sentinel",
        authorization: "Basic server-authorization-sentinel",
        cookie: "session=server-cookie-sentinel",
        raw_header: "X-API-Key: server-header-sentinel",
        safe_id: "17",
      },
    });

    const entry = JSON.parse(String(consoleLog.mock.calls[0]?.[0])) as {
      password: string;
      nested: Record<string, unknown>;
    };
    expect(entry).toMatchObject({
      password: "[REDACTED]",
      nested: {
        refresh_token: "[REDACTED]",
        jwt: "[REDACTED]",
        apiKey: "[REDACTED]",
        authorization: "[REDACTED]",
        cookie: "[REDACTED]",
        raw_header: "X-API-Key: [REDACTED]",
        safe_id: "17",
      },
    });
    expect(JSON.stringify(entry)).not.toMatch(
      /server-(password|token|jwt|api-key|authorization|cookie|header)-sentinel/,
    );
  });

  it("redacts child and call context before client batching", () => {
    vi.useFakeTimers();
    globalThis.window.history.replaceState(
      null,
      "",
      "/parents/enroll/status/route-token-sentinel/edit",
    );
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 200 }));
    globalThis.fetch = fetchMock;
    const logger = createLogger({
      component: "ClientProbe",
      level: "debug",
    }).child({ clientSecret: "client-secret-sentinel" });

    for (let index = 0; index < 10; index += 1) {
      logger.debug("client_redaction_probe", {
        device_pin: "client-pin-sentinel",
        jwt: "client-jwt-sentinel",
        api_key: "client-api-key-sentinel",
        raw_header: "X-API-Key: client-header-sentinel",
        headers: {
          Authorization: "Bearer client-authorization-sentinel",
          Cookie: "session=client-cookie-sentinel",
        },
        safe_index: index,
      });
    }

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = fetchMock.mock.calls[0]?.[1];
    expect(request).toMatchObject({ keepalive: true });
    const body = JSON.parse(String(request?.body)) as {
      entries: Array<Record<string, unknown>>;
    };
    expect(body.entries).toHaveLength(10);
    expect(body.entries[0]).toMatchObject({
      clientSecret: "[REDACTED]",
      device_pin: "[REDACTED]",
      jwt: "[REDACTED]",
      api_key: "[REDACTED]",
      raw_header: "X-API-Key: [REDACTED]",
      headers: {
        Authorization: "[REDACTED]",
        Cookie: "[REDACTED]",
      },
      safe_index: 0,
      route: "/parents/enroll/status/[REDACTED]/edit",
    });
    expect(JSON.stringify(body)).not.toMatch(
      /client-(secret|pin|jwt|api-key|authorization|cookie|header)-sentinel/,
    );
  });

  it("uses the redacted message for development console output", () => {
    vi.useFakeTimers();
    vi.stubEnv("NODE_ENV", "development");
    const consoleDebug = vi
      .spyOn(console, "debug")
      .mockImplementation(() => {});
    const logger = createLogger({
      component: "ClientConsoleProbe",
      level: "debug",
    });

    logger.debug(
      String.raw`request failed password="correct horse's \"quoted\" secret"`,
    );

    expect(consoleDebug).toHaveBeenCalledWith(
      "[ClientConsoleProbe]",
      `request failed password="${REDACTED_LOG_VALUE}"`,
      expect.any(String),
    );
  });
});
