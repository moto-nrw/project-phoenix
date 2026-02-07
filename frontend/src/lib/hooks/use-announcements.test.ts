import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import type { SWRResponse } from "swr";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockAuthFetch, mockPathname } = vi.hoisted(() => ({
  mockAuthFetch: vi.fn(),
  mockPathname: vi.fn(),
}));

vi.mock("~/lib/api-helpers", () => ({
  authFetch: mockAuthFetch,
}));

vi.mock("next/navigation", () => ({
  usePathname: (): ReturnType<typeof mockPathname> => mockPathname(),
}));

vi.mock("swr");

import { useAnnouncements } from "./use-announcements";
import useSWR from "swr";

interface UnreadAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

describe("useAnnouncements", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPathname.mockReturnValue("/dashboard");
  });

  it("returns empty array when no data", () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: undefined,
      mutate: vi.fn(),
      isLoading: true,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { result } = renderHook(() => useAnnouncements());

    expect(result.current.announcements).toEqual([]);
    expect(result.current.unreadCount).toBe(0);
    expect(result.current.isLoading).toBe(true);
  });

  it("returns announcements when data is loaded", () => {
    const mockData = [
      {
        id: 1,
        title: "Test Announcement",
        content: "Test content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-15T10:00:00Z",
      },
    ];

    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: mockData,
      mutate: vi.fn(),
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { result } = renderHook(() => useAnnouncements());

    expect(result.current.announcements).toEqual(mockData);
    expect(result.current.unreadCount).toBe(1);
    expect(result.current.isLoading).toBe(false);
  });

  it("calls mutate when pathname changes", async () => {
    const mockMutate = vi.fn();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: [],
      mutate: mockMutate,
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { rerender } = renderHook(() => useAnnouncements());

    mockPathname.mockReturnValue("/settings");
    rerender();

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("dismiss sends request to backend", async () => {
    mockAuthFetch.mockResolvedValueOnce({});

    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: [],
      mutate: vi.fn(),
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { result } = renderHook(() => useAnnouncements());

    await result.current.dismiss(123);

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/platform/announcements/123/dismiss",
      { method: "POST" },
    );
  });

  it("refresh calls mutate", () => {
    const mockMutate = vi.fn();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: [],
      mutate: mockMutate,
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { result } = renderHook(() => useAnnouncements());

    void result.current.refresh();

    expect(mockMutate).toHaveBeenCalled();
  });

  it("passes correct SWR options", () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: undefined,
      mutate: vi.fn(),
      isLoading: true,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    renderHook(() => useAnnouncements());

    expect(useSWR).toHaveBeenCalledWith(
      "user-announcements-unread",
      expect.any(Function),
      {
        refreshInterval: 60000,
        revalidateOnFocus: false,
        revalidateOnMount: true,
        dedupingInterval: 5000,
      },
    );
  });

  it("handles multiple announcements", () => {
    const mockData = [
      {
        id: 1,
        title: "Announcement 1",
        content: "Content 1",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-15T10:00:00Z",
      },
      {
        id: 2,
        title: "Announcement 2",
        content: "Content 2",
        type: "release",
        severity: "warning",
        version: "1.2.0",
        published_at: "2024-01-14T10:00:00Z",
      },
    ];

    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: mockData,
      mutate: vi.fn(),
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    const { result } = renderHook(() => useAnnouncements());

    expect(result.current.announcements).toHaveLength(2);
    expect(result.current.unreadCount).toBe(2);
  });

  it("does not mutate on first render with same pathname", () => {
    const mockMutate = vi.fn();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    vi.mocked(useSWR<UnreadAnnouncement[]>).mockReturnValue({
      data: [],
      mutate: mockMutate,
      isLoading: false,
      error: undefined,
      isValidating: false,
    } as SWRResponse<UnreadAnnouncement[]>);

    renderHook(() => useAnnouncements());

    // mutate should not be called on initial render
    // (useEffect only triggers on pathname changes, not initial value)
    expect(mockMutate).not.toHaveBeenCalled();
  });
});
