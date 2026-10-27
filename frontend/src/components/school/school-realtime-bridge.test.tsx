import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SchoolRealtimeBridge } from "./school-realtime-bridge";
import { CLASS_DAY_REFRESH_EVENT } from "~/components/class-day/use-class-day-refresh";
import type { SSEHookOptions } from "~/lib/sse-types";

const useSSE = vi.hoisted(() => vi.fn());

vi.mock("~/lib/hooks/use-sse", () => ({ useSSE }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { tenantId: "5", id: "11" } },
    status: "authenticated",
  }),
}));

function registeredHandler() {
  const options = useSSE.mock.calls.at(-1)?.[1] as SSEHookOptions | undefined;
  const onMessage = options?.onMessage;
  if (!onMessage) throw new Error("SSE handler was not registered");
  return onMessage;
}

describe("SchoolRealtimeBridge", () => {
  it("wakes an open class view after a class day changed (#4792)", () => {
    render(<SchoolRealtimeBridge />);

    expect(useSSE).toHaveBeenCalledWith(
      "/api/school/sse/events",
      expect.anything(),
    );
    const refresh = vi.fn();
    const teamChat = vi.fn();
    window.addEventListener(CLASS_DAY_REFRESH_EVENT, refresh);
    window.addEventListener("team-messages-unread-refresh", teamChat);

    try {
      registeredHandler()({
        type: "class_day_changed",
        active_group_id: "",
        data: { source: "parent" },
        timestamp: new Date().toISOString(),
      });

      expect(refresh).toHaveBeenCalledOnce();
      expect(teamChat).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener(CLASS_DAY_REFRESH_EVENT, refresh);
      window.removeEventListener("team-messages-unread-refresh", teamChat);
    }
  });

  it("keeps a team chat message away from the class view", () => {
    render(<SchoolRealtimeBridge />);

    const refresh = vi.fn();
    const teamChat = vi.fn();
    window.addEventListener(CLASS_DAY_REFRESH_EVENT, refresh);
    window.addEventListener("team-messages-unread-refresh", teamChat);

    try {
      registeredHandler()({
        type: "staff_message",
        active_group_id: "",
        data: { thread_id: "3" },
        timestamp: new Date().toISOString(),
      });

      expect(teamChat).toHaveBeenCalledOnce();
      expect(refresh).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener(CLASS_DAY_REFRESH_EVENT, refresh);
      window.removeEventListener("team-messages-unread-refresh", teamChat);
    }
  });
});
