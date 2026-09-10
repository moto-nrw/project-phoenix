import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const counts = vi.hoisted(() => ({
  parents: 0,
  team: 0,
  messagingEnabled: true,
  staffMessagingEnabled: true,
}));

vi.mock("~/lib/hooks/use-messages-unread", () => ({
  useMessagesUnread: () => ({ unreadCount: counts.parents }),
}));
vi.mock("~/lib/hooks/use-staff-messages-unread", () => ({
  useStaffMessagesUnread: () => ({ unreadCount: counts.team }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({
    tenant: {
      messagingEnabled: counts.messagingEnabled,
      staffMessagingEnabled: counts.staffMessagingEnabled,
    },
  }),
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { MessagesBlock } from "./messages-block";

describe("MessagesBlock (#2180)", () => {
  beforeEach(() => {
    counts.parents = 0;
    counts.team = 0;
    counts.messagingEnabled = true;
    counts.staffMessagingEnabled = true;
  });

  it("zählt ungelesene Nachrichten je Posteingang und führt dorthin", () => {
    counts.parents = 3;
    counts.team = 1;

    render(<MessagesBlock />);

    expect(screen.getByText("Nachrichten von Eltern")).toBeInTheDocument();
    expect(screen.getByText("3 ungelesen")).toBeInTheDocument();
    expect(screen.getByText("Team-Chat").closest("a")).toHaveAttribute(
      "href",
      "/test-tenant/team-chat",
    );
  });

  it("sagt es, wenn alles gelesen ist", () => {
    render(<MessagesBlock />);

    expect(screen.getByText("Alles gelesen")).toBeInTheDocument();
  });

  // Ein ausgeschalteter Posteingang zählt nicht, auch wenn der Zähler noch
  // einen alten Wert hält.
  it("lässt einen ausgeschalteten Posteingang weg", () => {
    counts.parents = 3;
    counts.messagingEnabled = false;

    render(<MessagesBlock />);

    expect(
      screen.queryByText("Nachrichten von Eltern"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Ungelesene Nachrichten: Posteingang öffnen",
      }),
    ).toHaveAttribute("href", "/test-tenant/team-chat");
  });
});
