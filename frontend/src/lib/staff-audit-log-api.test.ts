import { describe, expect, it } from "vitest";
import {
  buildAuditLogQuery,
  mapAuditLogEvent,
  type BackendAuditLogEvent,
} from "./staff-audit-log-api";

describe("mapAuditLogEvent", () => {
  it("maps ids to strings and keeps the typed detail payload", () => {
    const backend: BackendAuditLogEvent = {
      occurred_at: "2026-06-10T14:00:00+02:00",
      source: "session_edit",
      entry_id: 123,
      staff: { id: 42, name: "Anna Muster" },
      actor: {
        staff_id: 7,
        name: "Florian Admin",
        is_system: false,
        is_self: false,
      },
      reason: "Korrektur",
      detail: { session_id: 9, session_date: "2026-06-10", fields: [] },
    };

    const event = mapAuditLogEvent(backend);

    expect(event.staffId).toBe("42");
    expect(event.entryId).toBe("123");
    expect(event.staffName).toBe("Anna Muster");
    expect(event.actorStaffId).toBe("7");
    expect(event.actorIsSystem).toBe(false);
    expect(event.detail.session_date).toBe("2026-06-10");
  });

  it("keeps school-wide events without a staff and system actors without a staff id", () => {
    const backend: BackendAuditLogEvent = {
      occurred_at: "2026-06-10T14:00:00+02:00",
      source: "month_close",
      entry_id: 456,
      actor: { name: "System", is_system: true, is_self: false },
      reason: "",
      detail: { year: 2026, month: 5, account_count: 18 },
    };

    const event = mapAuditLogEvent(backend);

    expect(event.staffId).toBeNull();
    expect(event.actorStaffId).toBeNull();
    expect(event.actorIsSystem).toBe(true);
  });
});

describe("buildAuditLogQuery", () => {
  it("serializes every filter", () => {
    const query = buildAuditLogQuery({
      from: "2026-06-01",
      to: "2026-06-30",
      staffId: "42",
      actorId: "7",
      sources: ["session_edit", "deletion"],
      cursor: "abc",
      limit: 50,
    });
    const params = new URLSearchParams(query);
    expect(params.get("from")).toBe("2026-06-01");
    expect(params.get("to")).toBe("2026-06-30");
    expect(params.get("staff_id")).toBe("42");
    expect(params.get("actor_id")).toBe("7");
    expect(params.get("sources")).toBe("session_edit,deletion");
    expect(params.get("cursor")).toBe("abc");
    expect(params.get("limit")).toBe("50");
  });

  it("omits unset filters entirely", () => {
    expect(buildAuditLogQuery({})).toBe("");
  });
});
