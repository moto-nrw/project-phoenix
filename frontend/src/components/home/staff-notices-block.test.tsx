import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffNotice } from "~/lib/staff-notices-api";

const useSWRAuth = vi.hoisted(() => vi.fn());
const session = vi.hoisted(() => ({ accountId: "account-a" }));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { id: session.accountId } } }),
}));
vi.mock("~/components/home/home-card", () => ({
  HOME_CARD_BODY: "",
  HomeCardIcon: () => null,
}));
vi.mock("~/components/home/home-card-rows", () => ({
  HomeCardLink: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  HomeMoreRow: () => null,
  useHomeCardRows: () => ({ shown: [], hidden: 0 }),
}));
vi.mock("~/components/staff-notices/today-notice-list", () => ({
  TodayNoticeList: () => null,
}));
vi.mock("~/components/ui/section-card", () => ({
  SectionCard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("~/lib/staff-notices-api", () => ({ fetchTodaysNotices: vi.fn() }));
vi.mock("~/lib/swr", () => ({ useSWRAuth }));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

import { StaffNoticesBlock } from "./staff-notices-block";

describe("StaffNoticesBlock", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    session.accountId = "account-a";
    useSWRAuth.mockReturnValue({
      data: [] as StaffNotice[],
      error: undefined,
      isLoading: false,
      mutate: vi.fn(),
    });
  });

  it("trennt Hinweise nach angemeldetem Konto", () => {
    render(<StaffNoticesBlock />);

    expect(useSWRAuth).toHaveBeenCalledWith(
      "staff-notices-today:account-a",
      expect.any(Function),
      { revalidateOnFocus: false },
    );

    session.accountId = "account-b";
    render(<StaffNoticesBlock />);

    expect(useSWRAuth).toHaveBeenLastCalledWith(
      "staff-notices-today:account-b",
      expect.any(Function),
      { revalidateOnFocus: false },
    );
  });
});
