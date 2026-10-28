import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockGetConversation, mockPostMessage, mockListChildThreads } =
  vi.hoisted(() => ({
    mockGetConversation: vi.fn(),
    mockPostMessage: vi.fn(),
    mockListChildThreads: vi.fn(),
  }));

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-api")>()),
  getChildConversation: mockGetConversation,
  postChildMessage: mockPostMessage,
  listChildThreads: mockListChildThreads,
  getChildToday: vi.fn(),
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: () => undefined,
}));

vi.mock("~/components/parent/child-care", () => ({
  useChildCare: () => ({
    loading: false,
    features: { notes_enabled: true },
  }),
  getOgsActions: () => [],
  PickupTimeModal: () => null,
  SickNoteModal: () => null,
}));

import { OgsConversation } from "./ogs-conversation";

const base = {
  thread_id: "",
  student_id: "42",
  counterpart_name: "OGS",
  school_name: "Schule am Berg",
  student_name: "Felix Schneider",
  messages: [],
};

const teamView = {
  ...base,
  audience: "team",
  readers: {
    guardian: { name: "Sabine Schneider" },
    school: { kind: "all_staff", names: [] },
  },
  leadership_option: { available: true, names: ["Swantje M."] },
};

const leadershipView = {
  ...base,
  audience: "leadership",
  readers: {
    guardian: { name: "Sabine Schneider" },
    school: { kind: "leadership", names: ["Swantje M.", "Tom K."] },
  },
  leadership_option: { available: true, names: ["Swantje M.", "Tom K."] },
};

beforeEach(() => {
  vi.clearAllMocks();
  mockListChildThreads.mockResolvedValue([]);
});

// #5200: jede Unterhaltung sagt, wer mitliest; die Leitung ist wählbar.
describe("OgsConversation readers and audience", () => {
  it("says that all staff read the team conversation, in the head and at the composer", async () => {
    mockGetConversation.mockResolvedValue({
      ...teamView,
      leadership_option: { available: false, names: [] },
    });

    render(<OgsConversation studentId="42" />);

    const lines = await screen.findAllByText(
      "Lesen können: Sie und alle Mitarbeitenden der OGS.",
    );
    expect(lines).toHaveLength(2);
    // Without the school's offer and without a leadership conversation there
    // is nothing to choose.
    expect(screen.getByLabelText("Nachricht")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Nur die Leitung" }),
    ).not.toBeInTheDocument();
  });

  it("switches to the leadership, names it and sends there", async () => {
    mockGetConversation.mockImplementation((_id: string, audience?: string) =>
      Promise.resolve(audience === "leadership" ? leadershipView : teamView),
    );
    mockPostMessage.mockResolvedValue(leadershipView);

    render(<OgsConversation studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Nur die Leitung" }),
    );

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "OGS-Leitung der Schule am Berg",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Lesen können: Sie und die Leitung: Swantje M., Tom K.",
      ),
    ).toHaveLength(2);
    expect(mockGetConversation).toHaveBeenLastCalledWith("42", "leadership");

    fireEvent.change(screen.getByLabelText("Nachricht"), {
      target: { value: "Bitte um ein Gespräch" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    await waitFor(() =>
      expect(mockPostMessage).toHaveBeenCalledWith(
        "42",
        "Bitte um ein Gespräch",
        undefined,
        "leadership",
      ),
    );
  });

  it("keeps an existing leadership conversation reachable after the school switched the choice off", async () => {
    mockGetConversation.mockResolvedValue({
      ...teamView,
      leadership_option: { available: false, names: [] },
    });
    mockListChildThreads.mockResolvedValue([
      {
        thread_id: "7",
        student_id: "42",
        student_name: "Felix Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS",
        last_message_read_by_staff: false,
        unread: 2,
        audience: "leadership",
        readers: { school: { kind: "leadership", names: [] } },
      },
    ]);

    render(<OgsConversation studentId="42" />);

    expect(
      await screen.findByRole("button", { name: "Nur die Leitung (2 neu)" }),
    ).toBeInTheDocument();
  });
});

// #5200: ohne Leitung bleibt der Verlauf lesbar, Schreiben geht ans Team.
describe("OgsConversation without a leadership left", () => {
  it("keeps the history and points to the team instead of a composer", async () => {
    mockGetConversation.mockResolvedValue({
      ...leadershipView,
      thread_id: "7",
      readers: {
        guardian: { name: "Sabine Schneider" },
        school: { kind: "leadership", names: [] },
      },
      leadership_option: { available: false, names: [] },
      messages: [
        {
          id: "m1",
          kind: "message",
          sender_kind: "guardian",
          sender_name: "Sabine Schneider",
          body: "Bitte um ein Gespräch",
          created_at: "2026-09-09T08:00:00Z",
        },
      ],
    });

    render(<OgsConversation studentId="42" initialAudience="leadership" />);

    expect(
      await screen.findByText("Bitte um ein Gespräch"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Zurzeit liest niemand aus der Leitung mit. Bitte schreiben Sie an das ganze OGS-Team.",
      ).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Ganzes OGS-Team" }),
    ).toBeInTheDocument();
  });
});
