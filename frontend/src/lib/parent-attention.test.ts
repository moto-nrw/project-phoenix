import { describe, expect, it } from "vitest";

import type { AggregatedOpenRequest } from "~/lib/change-request-list-api";
import type { InboxThread } from "~/lib/parent-messages-api";
import { countParentAttention } from "~/lib/parent-attention";

function thread(studentId: string, unread: number): InboxThread {
  return {
    thread_id: `t-${studentId}-${unread}`,
    student_id: studentId,
    student_name: "Kind",
    guardian_name: "Mutter",
    unread_count: unread,
  };
}

function request(studentId: string): AggregatedOpenRequest {
  return {
    request_type: "excused",
    student_id: studentId,
    student_name: "Kind",
    occurred_at: "2026-09-09T08:00:00Z",
    data: {},
  } as unknown as AggregatedOpenRequest;
}

describe("countParentAttention", () => {
  it("adds unread messages of every conversation and open requests per child", () => {
    const counts = countParentAttention(
      [thread("1", 2), thread("1", 1), thread("2", 1)],
      [request("1"), request("3"), request("3")],
    );

    expect(counts.get("1")).toEqual({ messages: 3, requests: 1, total: 4 });
    expect(counts.get("2")).toEqual({ messages: 1, requests: 0, total: 1 });
    expect(counts.get("3")).toEqual({ messages: 0, requests: 2, total: 2 });
    expect(counts.size).toBe(3);
  });

  it("skips read conversations and requests without a child", () => {
    const enrollmentChange = {
      request_type: "enrollment",
      occurred_at: "2026-09-09T08:00:00Z",
      data: {},
    } as unknown as AggregatedOpenRequest;

    const counts = countParentAttention([thread("1", 0)], [enrollmentChange]);

    expect(counts.size).toBe(0);
    expect(counts.get("1")).toBeUndefined();
  });
});
