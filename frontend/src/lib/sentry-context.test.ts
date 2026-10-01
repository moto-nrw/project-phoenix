import { describe, expect, it, vi } from "vitest";

vi.mock("@sentry/nextjs", () => ({ setUser: vi.fn(), setTags: vi.fn() }));

const { sentrySessionContext } = await import("./sentry-context");

describe("sentrySessionContext", () => {
  it("gives an OGS staff session its account, role and school", () => {
    expect(
      sentrySessionContext("tenant", { id: "42", scope: "", tenantId: 12 }),
    ).toEqual({ user: { id: "42" }, role: "staff", schoolId: "12" });
  });

  it("names leadership, the staff preview and carrier sessions coarsely", () => {
    const admin = { id: "1", tenantId: 3, isAdmin: true };
    const preview = { id: "1", tenantId: 3, isPreview: true };
    const carrier = { id: "2", tenantId: 3, scope: "org" };

    expect(sentrySessionContext("tenant", admin).role).toBe("admin");
    expect(sentrySessionContext("tenant", preview).role).toBe("admin");
    expect(sentrySessionContext("tenant", carrier)).toEqual({
      user: { id: "2" },
      role: "carrier",
      schoolId: "3",
    });
  });

  it("binds a school portal session to its school", () => {
    expect(
      sentrySessionContext("school", { id: "5", scope: "school", tenantId: 8 }),
    ).toEqual({ user: { id: "5" }, role: "lehrkraft", schoolId: "8" });
  });

  it("leaves school_id out for operator sessions", () => {
    expect(
      sentrySessionContext("operator", { id: "7", scope: "platform" }),
    ).toEqual({ user: { id: "7" }, role: "operator", schoolId: null });
  });

  it("leaves school_id out for parent sessions, whatever the token carries", () => {
    expect(
      sentrySessionContext("parent", { id: "9", scope: "parent", tenantId: 0 }),
    ).toEqual({ user: { id: "9" }, role: "guardian", schoolId: null });
    expect(
      sentrySessionContext("parent", { id: "9", scope: "parent", tenantId: 4 })
        .schoolId,
    ).toBeNull();
  });

  it("has no school without a tenant ID and nothing without a session", () => {
    expect(sentrySessionContext("tenant", { id: "42" }).schoolId).toBeNull();
    expect(sentrySessionContext("tenant", null)).toEqual({
      user: null,
      role: null,
      schoolId: null,
    });
  });
});
