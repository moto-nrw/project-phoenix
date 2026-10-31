import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

// The send overlay (#5011) is bound to the signed-in session's scope.
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", scope: "parents" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
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

// #5011: die Nachricht steht sofort im Verlauf, markiert, bis die Antwort
// sie bestätigt; ein Fehler markiert sie als nicht gesendet.
describe("OgsConversation optimistic send", () => {
  const bubble = (text: string) => screen.getByText(text, { selector: "span" });

  it("shows the message within 100 ms, marked, and confirms it", async () => {
    mockGetConversation.mockResolvedValue(thread);
    let answer: (value: unknown) => void = () => {};
    mockPostMessage.mockReturnValueOnce(
      new Promise((resolve) => {
        answer = resolve;
      }),
    );

    render(<OgsConversation studentId="42" />);

    const field = await screen.findByLabelText("Nachricht");
    fireEvent.change(field, { target: { value: "Hallo OGS" } });
    const started = performance.now();
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    // Kein await: die Blase steht vor der Antwort.
    expect(bubble("Hallo OGS")).toBeInTheDocument();
    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
    expect(performance.now() - started).toBeLessThan(100);
    expect(field).toHaveValue("");

    answer({
      ...thread,
      messages: [
        {
          id: "m1",
          kind: "message",
          sender_kind: "guardian",
          sender_name: "Sabine Schneider",
          body: "Hallo OGS",
          created_at: "2026-09-09T10:00:00Z",
        },
      ],
    });
    await waitFor(() =>
      expect(screen.queryByText("Wird gesendet")).not.toBeInTheDocument(),
    );
    expect(screen.getAllByText("Hallo OGS", { selector: "span" })).toHaveLength(
      1,
    );
  });

  it("marks a failed message as not sent and puts the text back", async () => {
    mockGetConversation.mockResolvedValue(thread);
    mockPostMessage.mockRejectedValueOnce(
      new ApiError("weg", 503, { code: "general.unavailable" }),
    );

    render(<OgsConversation studentId="42" />);

    const field = await screen.findByLabelText("Nachricht");
    fireEvent.change(field, { target: { value: "Hallo OGS" } });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Nachricht"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(bubble("Hallo OGS")).toBeInTheDocument();
    expect(field).toHaveValue("Hallo OGS");
  });
});

// #5011: Ablehnungen der Route POST /api/parent/me/messages/children/{id}
// (child_write_errors.go) verwerfen den Verlauf, fachliche Zustände nicht.
describe("OgsConversation send refusals", () => {
  const history = {
    ...thread,
    messages: [
      {
        id: "s1",
        kind: "message",
        sender_kind: "staff",
        sender_name: "OGS",
        body: "Felix hat heute gut gegessen.",
        created_at: "2026-09-09T09:00:00Z",
      },
    ],
  };

  async function send() {
    render(<OgsConversation studentId="42" />);
    const field = await screen.findByLabelText("Nachricht");
    // Anker: der private Verlauf ist geladen.
    expect(
      screen.getByText("Felix hat heute gut gegessen."),
    ).toBeInTheDocument();
    fireEvent.change(field, { target: { value: "Hallo OGS" } });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));
  }

  it.each([
    "care.child_not_linked",
    "care.guardian_permission_denied",
  ] as const)("drops the history and the composer on 403 %s", async (code) => {
    const refusal = new ApiError("refused", 403, { code });
    mockGetConversation
      .mockResolvedValueOnce(history)
      .mockRejectedValue(refusal);
    mockPostMessage.mockRejectedValueOnce(refusal);
    await send();

    expect(
      await screen.findByText(catalogText(code, "die Unterhaltung")),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Felix hat heute gut gegessen."),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();
    expect(mockGetConversation).toHaveBeenCalledTimes(2);
  });

  it("keeps the history on 403 care.child_care_ended and marks the bubble", async () => {
    mockGetConversation.mockResolvedValue(history);
    mockPostMessage.mockRejectedValueOnce(
      new ApiError("ended", 403, { code: "care.child_care_ended" }),
    );
    await send();

    expect(await screen.findByText("Nicht gesendet")).toBeInTheDocument();
    expect(
      screen.getByText("Felix hat heute gut gegessen."),
    ).toBeInTheDocument();
    expect(mockGetConversation).toHaveBeenCalledTimes(1);
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
