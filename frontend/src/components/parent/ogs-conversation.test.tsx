import { fireEvent, render, screen } from "@testing-library/react";
import { ApiError } from "~/lib/api-error";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { catalogText } from "~/test/error-catalog-text";

const { mockGetConversation, mockPostMessage } = vi.hoisted(() => ({
  mockGetConversation: vi.fn(),
  mockPostMessage: vi.fn(),
}));

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-api")>()),
  getChildConversation: mockGetConversation,
  postChildMessage: mockPostMessage,
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

const thread = {
  counterpart_name: "OGS",
  school_name: "Schule am Berg",
  student_name: "Felix Schneider",
  messages: [],
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("OgsConversation back link", () => {
  it("names the inbox and sits in the head card above the title", async () => {
    mockGetConversation.mockResolvedValueOnce(thread);

    render(<OgsConversation studentId="42" showBack />);

    const title = await screen.findByRole("heading", {
      level: 1,
      name: "OGS-Team der Schule am Berg",
    });
    const back = screen.getByRole("link", { name: "Zurück zu Nachrichten" });
    expect(back).toHaveAttribute("href", "/parents/messages");
    expect(back.closest("section")).toBe(title.closest("section"));
    expect(
      back.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});

// #2518: Lade- und Sendefehler laufen über den gemeinsamen Fehlerweg, mit
// Katalogtext und Wiederholen, nie mit dem Satz vom Server.
describe("OgsConversation errors", () => {
  it("shows a failed load in the thread and retries it", async () => {
    mockGetConversation
      .mockRejectedValueOnce(
        new ApiError("thread kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(thread);

    render(<OgsConversation studentId="42" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/thread kaputt/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Noch keine Nachrichten", { exact: false }),
    ).toBeInTheDocument();
    expect(mockGetConversation).toHaveBeenCalledTimes(2);
  });

  it("shows a failed send above the composer and keeps the draft", async () => {
    mockGetConversation.mockResolvedValue(thread);
    mockPostMessage.mockRejectedValueOnce(
      new ApiError("not allowed", 403, { code: "general.permission" }),
    );

    render(<OgsConversation studentId="42" />);

    const field = await screen.findByLabelText("Nachricht");
    fireEvent.change(field, { target: { value: "Hallo OGS" } });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Nachricht"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/not allowed/)).not.toBeInTheDocument();
    expect(field).toHaveValue("Hallo OGS");
  });
});

// #4784: eine Antwort auf eine Gruppennachricht zeigt, worauf sie sich bezieht.
describe("OgsConversation group thread answers", () => {
  it("shows the group thread an own answer refers to and links to it", async () => {
    mockGetConversation.mockResolvedValueOnce({
      ...thread,
      messages: [
        {
          id: "m1",
          kind: "message",
          sender_kind: "guardian",
          sender_name: "Sabine Schneider",
          body: "Wir bringen einen Apfelkuchen mit.",
          created_at: "2026-09-09T08:00:00Z",
          group_thread_id: "9",
          group_thread_title: "Sommerfest am Freitag",
        },
        {
          id: "m2",
          kind: "message",
          sender_kind: "guardian",
          sender_name: "Sabine Schneider",
          body: "Und noch eine Frage.",
          created_at: "2026-09-09T08:05:00Z",
        },
      ],
    });

    render(<OgsConversation studentId="42" />);

    const reference = await screen.findByRole("link", {
      name: "Antwort auf: Sommerfest am Freitag",
    });
    expect(reference).toHaveAttribute("href", "/parents/messages/group/9");
    expect(screen.getByText("Und noch eine Frage.")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Antwort auf/ })).toHaveLength(
      1,
    );
  });
});
