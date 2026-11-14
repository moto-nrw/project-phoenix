import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useStudentParentAttention } from "./use-parent-attention";
import type { InboxThread } from "~/lib/parent-messages-api";
import type { AggregatedOpenRequest } from "~/lib/change-request-list-api";
import type { ParentAttention } from "~/lib/parent-attention";

const mocks = vi.hoisted(() => ({
  booked: true,
  reviewsRequests: true,
  interval: 60000,
  inbox: vi.fn<() => Promise<InboxThread[]>>(),
  requests: vi.fn<() => Promise<AggregatedOpenRequest[]>>(),
  mutate: vi.fn(),
  liveInterval: vi.fn(),
  swr: vi.fn<
    (
      key: string | null,
      fetcher: () => Promise<ReadonlyMap<string, ParentAttention>>,
      options: {
        refreshInterval?: number;
        revalidateOnFocus?: boolean;
        keepPreviousData?: boolean;
      },
    ) => { data: undefined; mutate: () => void }
  >(),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "staff" } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/auth-utils", () => ({
  isAdmin: () => false,
  hasPermission: () => true,
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({ tenant: { parentMessageCardBadgeEnabled: true } }),
  useModuleBooked: () => mocks.booked,
}));
vi.mock("~/lib/hooks/use-change-request-access", () => ({
  useChangeRequestAccess: () => ({
    canReviewParentRequests: mocks.reviewsRequests,
    isLoading: false,
  }),
}));
vi.mock("~/lib/parent-messages-api", () => ({
  fetchInboxWithFilters: mocks.inbox,
}));
vi.mock("~/lib/parent-attention", async (original) => ({
  ...(await original<typeof import("~/lib/parent-attention")>()),
  fetchAllOpenRequests: mocks.requests,
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.swr,
  useLiveRefreshInterval: () => {
    mocks.liveInterval();
    return mocks.interval;
  },
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.booked = true;
  mocks.reviewsRequests = true;
  mocks.interval = 60000;
  mocks.inbox.mockResolvedValue([]);
  mocks.requests.mockResolvedValue([
    {
      request_type: "offering",
      student_id: "7",
      student_name: "Mia",
      occurred_at: "2026-09-09",
      data: {},
    } as unknown as AggregatedOpenRequest,
  ]);
  mocks.swr.mockReturnValue({ data: undefined, mutate: mocks.mutate });
});

describe("parent attention availability and live refresh", () => {
  it("retains independent request counts when the Eltern-App is not booked", async () => {
    mocks.booked = false;
    renderHook(() => useStudentParentAttention("7"));
    const fetcher = mocks.swr.mock.calls[0]?.[1];
    expect(fetcher).toBeDefined();
    const result = await fetcher?.();
    expect(mocks.inbox).not.toHaveBeenCalled();
    expect(mocks.requests).toHaveBeenCalledOnce();
    expect(result?.get("7")).toEqual({ messages: 0, requests: 1, total: 1 });
  });

  it("does not request data when neither source is available", () => {
    mocks.booked = false;
    mocks.reviewsRequests = false;
    renderHook(() => useStudentParentAttention("7"));
    expect(mocks.swr.mock.calls[0]?.[0]).toBeNull();
  });

  it("uses bounded live-aware polling so a focused list can recover lost events", () => {
    const { rerender } = renderHook(() => useStudentParentAttention("7"));
    expect(mocks.swr.mock.lastCall?.[2].refreshInterval).toBe(60000);
    mocks.interval = 300000;
    rerender();
    expect(mocks.swr.mock.lastCall?.[2].refreshInterval).toBe(300000);
    expect(mocks.liveInterval).toHaveBeenCalled();
  });
});
