import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TenantInfo } from "~/lib/tenant-api";

const { mockApiGet } = vi.hoisted(() => ({ mockApiGet: vi.fn() }));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
}));

vi.mock("~/lib/user-context.server", () => ({
  loadUserContext: vi.fn(async () => ({
    educationalGroups: [],
    supervisedGroups: [],
    currentStaff: null,
    educationalGroupIds: [],
    educationalGroupRoomNames: [],
    supervisedRoomNames: [],
    incomplete: false,
    unavailableSections: [],
  })),
}));

import { loadShellBootstrap } from "./shell-bootstrap.server";
import { loadUserContext } from "~/lib/user-context.server";

function session(overrides: Partial<Session["user"]> = {}): Session {
  return {
    user: {
      id: "7",
      name: "Admin",
      email: "admin@example.com",
      token: "access-token",
      roles: ["admin"],
      permissions: ["admin:*"],
      tenantId: 1,
      ...overrides,
    },
    expires: "2099-01-01",
  };
}

const tenant = {
  tenantId: 1,
  slug: "school-a",
  subdomain: "school-a",
  messagingEnabled: true,
  staffMessagingEnabled: false,
  operationalOverviewScope: "own",
} as TenantInfo;

const calledEndpoints = () =>
  mockApiGet.mock.calls.map((call) => call[0] as string).sort();

function routeBackend(responses: Record<string, unknown | Error>): void {
  mockApiGet.mockImplementation(async (endpoint: string) => {
    const response = responses[endpoint];
    if (response instanceof Error) throw response;
    if (response === undefined) throw new Error(`unrouted ${endpoint}`);
    return response;
  });
}

beforeEach(() => {
  mockApiGet.mockReset();
  vi.mocked(loadUserContext).mockClear();
});

describe("loadShellBootstrap", () => {
  it("loads every shell field an admin's sidebar renders, in one pass", async () => {
    routeBackend({
      "/api/settings/schema": { data: { tabs: [] } },
      "/api/me/profile": {
        data: { id: 7, first_name: "Ada", last_name: "L", email: "a@b" },
      },
      "/auth/account/tenants": {
        data: [
          {
            tenant_id: 1,
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_id: 3,
            organization_name: "Org",
          },
        ],
      },
      "/api/students/ogs-group-navigation": {
        data: [{ id: "2", name: "Zebra" }],
      },
      "/api/active/supervisors/all": {
        data: [
          { id: 1, group_id: 1, room_id: 5, room: { id: 5, name: "Aula" } },
        ],
      },
      "/api/active/schulhof/status": {
        data: {
          exists: true,
          room_name: "Schulhof",
          is_user_supervising: false,
        },
      },
      "/api/staff/absences/pending": { data: [{}, {}] },
      "/api/messages/unread-count": { data: { unread_count: 4 } },
      "/api/staff-notices/today": {
        data: [
          { requires_acknowledgement: true },
          { requires_acknowledgement: true, acknowledged_at: "2026-09-01" },
          { requires_acknowledgement: false },
        ],
      },
      "/api/students/change-requests/pending-count": {
        data: { pending_count: 3 },
      },
      "/api/enrollment/admin/change-requests/pending-count": {
        data: { pending_count: 1 },
      },
      "/api/students/care-withdrawals?page_size=1": { data: { total: 5 } },
      "/api/reminders": { data: { enabled: true, reminders: [], count: 0 } },
      "/api/platform/announcements/unread": { data: [{ id: 1 }] },
    });

    const shell = await loadShellBootstrap(session(), tenant);

    expect(shell.accountId).toBe("7");
    expect(shell.reminders).toEqual({
      enabled: true,
      reminders: [],
      count: 0,
    });
    expect(shell.announcements).toEqual([{ id: 1 }]);
    expect(loadUserContext).toHaveBeenCalledWith(
      "access-token",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(shell.userContext?.incomplete).toBe(false);
    expect(shell.settingsSchema).toEqual({ tabs: [] });
    expect(shell.profile?.firstName).toBe("Ada");
    expect(shell.accountTenants?.[0]?.organizationName).toBe("Org");
    expect(shell.supervision).toEqual({
      groups: [{ id: "2", name: "Zebra" }],
      supervised: [
        { id: 1, group_id: 1, room_id: 5, room: { id: 5, name: "Aula" } },
      ],
      schulhof: {
        exists: true,
        room_name: "Schulhof",
        is_user_supervising: false,
      },
      overviewOk: true,
    });
    expect(shell.counts).toEqual({
      staffAbsencesPending: 2,
      messagesUnread: 4,
      teamChatUnread: undefined,
      staffNoticesPending: 1,
      changeRequestsPending: 3,
      enrollmentRequestsPending: 1,
      careWithdrawalsPending: 5,
    });
    // Team chat is off for this tenant: not even requested.
    expect(calledEndpoints()).not.toContain("/api/staff-messages/unread-count");
  });

  it("skips the calls a caregiver's hooks would not make, and keeps the rest", async () => {
    routeBackend({
      "/api/me/profile": {
        data: { id: 8, first_name: "Bo", last_name: "K", email: "b@k" },
      },
      "/auth/account/tenants": { data: [] },
      "/api/students/ogs-group-navigation": { data: [] },
      "/api/me/groups/supervised": { data: [] },
      "/api/active/schulhof/status": { data: { exists: false } },
      "/api/staff-notices/today": { data: [] },
      "/api/messages/unread-count": { data: { unread_count: 2 } },
      "/api/reminders": { data: { enabled: false, reminders: [], count: 0 } },
      "/api/platform/announcements/unread": { data: null },
    });

    const shell = await loadShellBootstrap(
      session({
        roles: ["teacher"],
        permissions: ["groups:read", "users:read"],
      }),
      tenant,
    );

    expect(shell.settingsSchema).toBeUndefined();
    expect(shell.supervision?.overviewOk).toBe(false);
    expect(shell.counts).toEqual({
      staffAbsencesPending: undefined,
      // The messages badge is gated on the tenant flag only, not on a
      // permission, so a caregiver's count is loaded too.
      messagesUnread: 2,
      teamChatUnread: undefined,
      staffNoticesPending: 0,
      changeRequestsPending: undefined,
      enrollmentRequestsPending: undefined,
      careWithdrawalsPending: undefined,
    });
    const endpoints = calledEndpoints();
    expect(endpoints).not.toContain("/api/settings/schema");
    expect(endpoints).not.toContain("/api/active/supervisors/all");
    expect(endpoints).not.toContain("/api/staff/absences/pending");
  });

  it("falls back to the own-supervision endpoint when the overview is refused", async () => {
    routeBackend({
      "/api/me/profile": new Error("500"),
      "/auth/account/tenants": new Error("500"),
      "/api/settings/schema": new Error("500"),
      "/api/students/ogs-group-navigation": new Error("500"),
      "/api/active/supervisors/all": new Error("API error (403)"),
      "/api/me/groups/supervised": {
        data: [{ id: 3, group_id: 3, room_id: 2, room: { id: 2, name: "B" } }],
      },
      "/api/active/schulhof/status": new Error("500"),
      "/api/staff/absences/pending": new Error("500"),
      "/api/messages/unread-count": new Error("500"),
      "/api/staff-notices/today": new Error("500"),
      "/api/students/change-requests/pending-count": new Error("500"),
      "/api/enrollment/admin/change-requests/pending-count": new Error("500"),
      "/api/students/care-withdrawals?page_size=1": new Error("500"),
      "/api/reminders": new Error("500"),
      "/api/platform/announcements/unread": new Error("500"),
    });
    vi.mocked(loadUserContext).mockRejectedValueOnce(new Error("500"));

    const shell = await loadShellBootstrap(session(), tenant);

    expect(shell.userContext).toBeUndefined();
    expect(shell.profile).toBeUndefined();
    expect(shell.settingsSchema).toBeUndefined();
    expect(shell.accountTenants).toBeUndefined();
    expect(shell.reminders).toBeUndefined();
    expect(shell.announcements).toBeUndefined();
    expect(shell.supervision).toBeNull();
    expect(Object.values(shell.counts).every((v) => v === undefined)).toBe(
      true,
    );
  });

  it("leaves an incomplete navigation projection to the browser", async () => {
    routeBackend({});
    vi.mocked(loadUserContext).mockResolvedValueOnce({
      educationalGroups: [],
      supervisedGroups: [],
      currentStaff: null,
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
      incomplete: true,
      unavailableSections: ["supervised_groups"],
    });

    const shell = await loadShellBootstrap(session(), tenant);

    expect(shell.userContext).toBeUndefined();
  });

  it("aborts timed-out backend preloads", async () => {
    vi.useFakeTimers();
    let profileSignal: AbortSignal | undefined;
    mockApiGet.mockImplementation(
      (
        endpoint: string,
        _token: string,
        options?: { signal?: AbortSignal },
      ) => {
        if (endpoint !== "/api/me/profile") {
          throw new Error(`unrouted ${endpoint}`);
        }
        profileSignal = options?.signal;
        return new Promise((_, reject) => {
          profileSignal?.addEventListener("abort", () => {
            reject(new Error("aborted"));
          });
        });
      },
    );

    try {
      const shellPromise = loadShellBootstrap(session(), tenant);
      await vi.advanceTimersByTimeAsync(1500);
      const shell = await shellPromise;

      expect(profileSignal?.aborted).toBe(true);
      expect(shell.profile).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });
});
