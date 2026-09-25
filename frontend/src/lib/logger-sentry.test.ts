import { beforeEach, describe, expect, it, vi } from "vitest";

const { addBreadcrumb, captureMessage } = vi.hoisted(() => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
}));
vi.mock("@sentry/nextjs", () => ({ addBreadcrumb, captureMessage }));

vi.unmock("~/lib/logger");

const { reportLogToSentry } = await import("./logger-sentry");
const { createLogger } = await import("./logger");

function entry(overrides: Record<string, unknown>) {
  return {
    timestamp: "2026-09-23T14:15:08.000Z",
    level: "error" as const,
    msg: "parent_news_poll_answer_failed",
    component: "ParentNews",
    environment: "production" as const,
    context: "client" as const,
    ...overrides,
  };
}

describe("reportLogToSentry", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("turns a handled client error into a Sentry event grouped by message", () => {
    reportLogToSentry(entry({ error: "Failed to fetch", route: "/news" }));

    expect(captureMessage).toHaveBeenCalledWith(
      "parent_news_poll_answer_failed",
      {
        level: "error",
        tags: { component: "ParentNews", log_source: "logger" },
        extra: { error: "Failed to fetch", route: "/news" },
        fingerprint: ["logger", "ParentNews", "parent_news_poll_answer_failed"],
      },
    );
  });

  it("records client info and warnings only as breadcrumbs", () => {
    reportLogToSentry(entry({ level: "info", msg: "poll_opened" }));
    reportLogToSentry(entry({ level: "warn", msg: "poll_slow" }));

    expect(captureMessage).not.toHaveBeenCalled();
    expect(addBreadcrumb).toHaveBeenCalledTimes(2);
    expect(addBreadcrumb).toHaveBeenLastCalledWith(
      expect.objectContaining({
        category: "log.ParentNews",
        message: "poll_slow",
        level: "warning",
      }),
    );
  });

  it("keeps debug output out of Sentry", () => {
    reportLogToSentry(entry({ level: "debug" }));

    expect(addBreadcrumb).not.toHaveBeenCalled();
    expect(captureMessage).not.toHaveBeenCalled();
  });

  it("does not send expected noise as events", () => {
    reportLogToSentry(entry({ msg: "sse connection error" }));
    reportLogToSentry(entry({ msg: "parent login failed", context: "server" }));
    reportLogToSentry(entry({ msg: "login failed" }));
    reportLogToSentry(entry({ msg: "school login failed", context: "server" }));

    expect(captureMessage).not.toHaveBeenCalled();
  });

  it("sends server errors as events without breadcrumbs", () => {
    reportLogToSentry(entry({ msg: "api route error", context: "server" }));

    expect(addBreadcrumb).not.toHaveBeenCalled();
    expect(captureMessage).toHaveBeenCalledTimes(1);
  });

  it("tags an event with the error code and Vorgangskennung the entry carries", () => {
    reportLogToSentry(
      entry({
        msg: "api route error",
        component: "ApiHelpers",
        context: "server",
        status: 503,
        error_code: "db_unavailable",
        request_id: "0b6f3f4e-5c1d-4a52-9d57-2d3c1b5e8f10",
      }),
    );

    expect(captureMessage).toHaveBeenCalledWith(
      "api route error",
      expect.objectContaining({
        tags: {
          component: "ApiHelpers",
          log_source: "logger",
          error_code: "db_unavailable",
          request_id: "0b6f3f4e-5c1d-4a52-9d57-2d3c1b5e8f10",
        },
      }),
    );
  });
});

describe("logger levels for expected failures (#3694)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it.each([
    ["a dropped connection", { error: "Load failed" }],
    ["a prefixed dropped connection", { error: "TypeError: Failed to fetch" }],
    ["an expired session", { status: 401, error: "unauthorized" }],
    ["a business-rule conflict", { status: 409, error: "request_past" }],
    [
      "a conflict named only in the API error text",
      { error: "API error (409): room still in use" },
    ],
  ])("logs %s as a warning breadcrumb, not an event", (_label, context) => {
    createLogger({ component: "Probe" }).error("probe_failed", context);

    expect(captureMessage).not.toHaveBeenCalled();
    expect(addBreadcrumb).toHaveBeenCalledWith(
      expect.objectContaining({
        message: "probe_failed",
        level: "warning",
        data: expect.objectContaining({
          expected_failure: expect.any(String),
        }) as unknown,
      }),
    );
  });

  it.each([
    ["a 403", { status: 403, error: "timetable operation forbidden" }],
    ["a 5xx", { status: 503, error: "API error (503): unavailable" }],
    ["an exception", { error: "Cannot read properties of undefined" }],
    [
      "an unreadable response",
      { error: "SyntaxError: The string did not match the expected pattern." },
    ],
  ])("still sends %s as an event", (_label, context) => {
    createLogger({ component: "Probe" }).error("probe_failed", context);

    expect(captureMessage).toHaveBeenCalledWith(
      "probe_failed",
      expect.objectContaining({ level: "error" }),
    );
  });
});
