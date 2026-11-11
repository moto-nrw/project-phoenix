// #6593: Während die Unterhaltung geöffnet wird, ist das Panel gesperrt; eine
// späte Antwort öffnet keinen Chat, wenn das Panel zu ist oder inzwischen ein
// anderer Leserkreis gewählt wurde. Echter SlideOver über dem gemeinsamen
// Vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { ThreadDetail } from "~/lib/parent-messages-api";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { mockFetchGuardians, mockOpenThread } = vi.hoisted(() => ({
  mockFetchGuardians: vi.fn(),
  mockOpenThread: vi.fn(),
}));
vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  fetchGuardians: mockFetchGuardians,
  fetchReachableStudentIds: vi.fn(),
  openThread: mockOpenThread,
}));
vi.mock("~/lib/student-api", () => ({ fetchStudents: vi.fn() }));

import { NewMessageModal } from "./new-message-modal";

const TITLE = "Neue Nachricht";
const OPEN_ERROR = catalogText("general.server", "die Unterhaltung");

const anna = {
  account_id: "7",
  name: "Anna Muster",
  relationship_type: "parent",
  is_primary: true,
  leadership_available: true,
};

const thread = {
  thread_id: "t1",
  student_id: "42",
  student_name: "Max Muster",
  guardian_name: "Anna Muster",
  messages: [],
} as unknown as ThreadDetail;

async function renderAndOpen() {
  const answer = deferred<ThreadDetail>();
  mockOpenThread.mockReturnValueOnce(answer.promise);
  const onClose = vi.fn();
  const onOpened = vi.fn();
  const view = render(
    <NewMessageModal
      onClose={onClose}
      onOpened={onOpened}
      presetStudentId="42"
      presetStudentName="Max Muster"
    />,
  );
  const row = (await screen.findByText("Anna Muster")).closest("button");
  expect(row).toBeEnabled();
  fireEvent.click(row as HTMLElement);
  return { ...view, answer, onClose, onOpened };
}

describe("NewMessageModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchGuardians.mockResolvedValue([anna]);
  });

  it("locks the panel while opening and unlocks it after a failure", async () => {
    const { answer, onClose, onOpened } = await renderAndOpen();

    // Schritt 2 hat keine Fußleiste; die Elternzeile sperrt mit.
    expectSlideOverSaveLocked(TITLE, []);
    expect(screen.getByText("Öffnen...")).toBeInTheDocument();
    expect(screen.getByText("Anna Muster").closest("button")).toBeDisabled();
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(await screen.findByText(OPEN_ERROR)).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByText("Anna Muster").closest("button")).toBeEnabled();
    expect(onOpened).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("opens the chat and closes after success", async () => {
    const { answer, onClose, onOpened } = await renderAndOpen();

    await act(async () => {
      answer.resolve(thread);
    });

    expect(onOpened).toHaveBeenCalledWith(thread);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("opens no chat when the answer arrives after unmount", async () => {
    const { answer, onClose, onOpened, unmount } = await renderAndOpen();
    expect(mockOpenThread).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve(thread);
    });

    expect(onOpened).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("opens no chat for the audience left meanwhile", async () => {
    const { answer, onClose, onOpened } = await renderAndOpen();
    expect(mockOpenThread).toHaveBeenCalledWith(
      expect.objectContaining({ audience: "team" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Nur Leitung" }));
    expect(screen.getByRole("button", { name: "Nur Leitung" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(await screen.findByText("Anna Muster")).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.resolve(thread);
    });

    expect(onOpened).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
