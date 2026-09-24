import { beforeEach, describe, expect, it, vi } from "vitest";

const { addBreadcrumb, captureMessage } = vi.hoisted(() => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
}));
vi.mock("@sentry/nextjs", () => ({ addBreadcrumb, captureMessage }));

const { reportLogToSentry } = await import("./logger-sentry");

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

    expect(captureMessage).not.toHaveBeenCalled();
  });

  it("sends server errors as events without breadcrumbs", () => {
    reportLogToSentry(entry({ msg: "api route error", context: "server" }));

    expect(addBreadcrumb).not.toHaveBeenCalled();
    expect(captureMessage).toHaveBeenCalledTimes(1);
  });
});
