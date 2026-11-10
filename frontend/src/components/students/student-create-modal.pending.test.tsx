// #6593: Während ein Kind oder ein Klassenlisteneintrag angelegt wird, bleibt
// das Panel zu; eine späte Antwort landet in keinem neueren Vorgang. Echter
// SlideOver mit vaul-Mock, echte Formularfelder.
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { mockFetchArrivalSettings, mockFetchPrivacyConsentDefaults } =
  vi.hoisted(() => ({
    mockFetchArrivalSettings: vi.fn(),
    mockFetchPrivacyConsentDefaults: vi.fn(),
  }));
vi.mock("~/lib/student-api", () => ({
  fetchPrivacyConsentDefaults: mockFetchPrivacyConsentDefaults,
}));
vi.mock("~/lib/student-arrival-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-arrival-api")>()),
  fetchArrivalSettings: mockFetchArrivalSettings,
}));

import { StudentCreateModal } from "./student-create-modal";

const TITLE = "Neues Kind";
const CLOSE = "Fenster schließen";
const LOCKED_FOOTER = ["Abbrechen", "Wird erstellt..."] as const;

type Props = ComponentProps<typeof StudentCreateModal>;

function renderModal(props: Partial<Props> = {}) {
  const onClose = vi.fn();
  const modal = (next: Partial<Props> = {}) => (
    <StudentCreateModal
      isOpen
      onClose={onClose}
      onCreate={vi.fn().mockResolvedValue(undefined)}
      {...props}
      {...next}
    />
  );
  const view = render(modal());
  return { ...view, modal, onClose };
}

function fillName() {
  fireEvent.change(screen.getByPlaceholderText("Max"), {
    target: { value: "Mia" },
  });
  fireEvent.change(screen.getByPlaceholderText("Mustermann"), {
    target: { value: "Muster" },
  });
  fireEvent.change(screen.getByPlaceholderText("5A"), {
    target: { value: "2a" },
  });
}

async function createListEntry() {
  fireEvent.click(screen.getByRole("button", { name: "Nur Klassenliste" }));
  fillName();
  const create = screen.getByRole("button", { name: "Erstellen" });
  expect(create).toBeEnabled();
  fireEvent.click(create);
}

describe("StudentCreateModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchArrivalSettings.mockResolvedValue({
      careDaysSource: "weekly_plan",
      schoolPeriods: [],
    });
    mockFetchPrivacyConsentDefaults.mockResolvedValue({
      accepted: false,
      dataRetentionDays: 14,
    });
  });

  it("keeps the panel shut while creating a child and unlocks it after a failure", async () => {
    const answer = deferred();
    const onCreate = vi.fn(() => answer.promise);
    const { onClose } = renderModal({ onCreate });
    const create = await screen.findByRole("button", { name: "Erstellen" });
    fillName();
    expect(create).toBeEnabled();
    fireEvent.click(create);
    expect(onCreate).toHaveBeenCalledOnce();

    expectSlideOverSaveLocked(TITLE, LOCKED_FOOTER, CLOSE);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Das Kind ist gerade nicht erreichbar.",
    );
    expectSlideOverUnlocked(TITLE, CLOSE);
    expect(screen.getByPlaceholderText("Max")).toHaveValue("Mia");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes the panel after a class list entry was created", async () => {
    const answer = deferred();
    const onCreateListEntry = vi.fn(() => answer.promise);
    const { onClose } = renderModal({ onCreateListEntry });
    await createListEntry();

    expectSlideOverSaveLocked(TITLE, LOCKED_FOOTER, CLOSE);
    expect(onCreateListEntry).toHaveBeenCalledWith({
      firstName: "Mia",
      lastName: "Muster",
      schoolClass: "2a",
    });

    await act(async () => {
      answer.resolve();
    });

    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not close a reopened panel when the old entry answers late", async () => {
    const answer = deferred();
    const { onClose, rerender, modal } = renderModal({
      onCreateListEntry: vi.fn(() => answer.promise),
    });
    await createListEntry();

    rerender(modal({ isOpen: false }));
    rerender(modal());
    expectSlideOverUnlocked(TITLE, CLOSE);

    await act(async () => {
      answer.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: TITLE })).toBeVisible();
  });

  it("calls no close for an entry that answers after unmount", async () => {
    const answer = deferred();
    const onCreateListEntry = vi.fn(() => answer.promise);
    const { onClose, unmount } = renderModal({ onCreateListEntry });
    await createListEntry();
    expect(onCreateListEntry).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
  });
});
