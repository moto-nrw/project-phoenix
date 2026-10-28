import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { shellSwrConfig } from "~/app/[tenant]/providers";
import type { ShellBootstrap } from "~/lib/shell-seed";
import { SWRConfig } from "~/lib/swr/cache";
import { resetSWRScope } from "~/lib/swr/scope";
import { useAnnouncements } from "./use-announcements";

vi.unmock("swr");
const { authFetch, owner } = vi.hoisted(() => ({
  authFetch: vi.fn(),
  owner: { id: "7" },
}));
vi.mock("~/lib/api-helpers", () => ({ authFetch }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: owner, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

const seed = [
  {
    id: 1,
    title: "Release",
    content: "News",
    type: "release",
    severity: "info",
    published_at: "2026-09-09T10:00:00Z",
  },
];
const shell: ShellBootstrap = {
  accountId: "7",
  counts: {},
  announcements: seed,
};
function Wrapper({ children }: { children: React.ReactNode }) {
  return (
    <SWRConfig value={shellSwrConfig(shell, "school-a")}>{children}</SWRConfig>
  );
}

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  owner.id = "7";
  authFetch.mockReset().mockResolvedValue({ data: [] });
});
afterEach(() => resetSWRScope({ revalidate: false }));

it("shows bootstrap announcements without a redundant request, then refreshes from the server", async () => {
  const { result } = renderHook(() => useAnnouncements(), { wrapper: Wrapper });
  expect(result.current.announcements).toEqual(seed);
  await act(async () => {});
  expect(authFetch).toHaveBeenCalledTimes(0);

  await act(async () => {
    await result.current.refresh();
  });
  expect(authFetch).toHaveBeenCalledTimes(1);
  expect(authFetch).toHaveBeenCalledWith("/api/platform/announcements/unread");
  expect(result.current.announcements).toEqual([]);
});

it("does not show another account's bootstrap announcements", async () => {
  owner.id = "8";
  const { result } = renderHook(() => useAnnouncements(), { wrapper: Wrapper });
  expect(result.current.announcements).toEqual([]);
  await act(async () => {});
  expect(authFetch).toHaveBeenCalledTimes(1);
  expect(result.current.announcements).toEqual([]);
});
