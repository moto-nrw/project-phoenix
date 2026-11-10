// #6593: Während die Erziehungsberechtigten gespeichert werden, ist das Panel
// gesperrt; eine späte Antwort schließt kein Panel, das inzwischen eine andere
// Person zeigt. Echter SlideOver über dem gemeinsamen Vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

import GuardianFormModal from "./guardian-form-modal";

const TITLE = "Erziehungsberechtigte/n bearbeiten";
const FOOTER = ["Entfernen", "Abbrechen", /Wird gespeichert/];
const SAVE_ERROR = catalogText(
  "general.server",
  "das Speichern der Erziehungsberechtigten",
);

function guardian(
  id: string,
  firstName: string,
  lastName: string,
): GuardianWithRelationship {
  return {
    id,
    firstName,
    lastName,
    email: `${firstName.toLowerCase()}@example.com`,
    preferredContactMethod: "email",
    languagePreference: "de",
    hasAccount: false,
    phoneNumbers: [],
    relationshipType: "parent",
    isEmergencyContact: true,
    isPrimary: true,
    canPickup: true,
    emergencyPriority: 1,
    relationshipId: `rel-${id}`,
  };
}

const max = guardian("g1", "Max", "Mustermann");
const erika = guardian("g2", "Erika", "Beispiel");

type Props = Parameters<typeof GuardianFormModal>[0];

function renderAndSave() {
  const answer = deferred();
  const props: Props = {
    isOpen: true,
    onClose: vi.fn(),
    onSubmit: vi.fn(() => answer.promise),
    onDelete: vi.fn(),
    mode: "edit",
    initialData: max,
  };
  const view = render(<GuardianFormModal {...props} />);
  fireEvent.change(screen.getByDisplayValue("Max"), {
    target: { value: "Maxi" },
  });
  const save = screen.getByRole("button", { name: "Speichern" });
  expect(save).toBeEnabled();
  fireEvent.click(save);
  return { ...view, answer, props };
}

describe("GuardianFormModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("locks the panel while saving and unlocks it after a failure", async () => {
    const { answer, props } = renderAndSave();

    expectSlideOverSaveLocked(TITLE, FOOTER);
    expect(props.onClose).not.toHaveBeenCalled();
    expect(props.onDelete).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(await screen.findByText(SAVE_ERROR)).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByDisplayValue("Maxi")).toBeInTheDocument();
    expect(props.onClose).not.toHaveBeenCalled();
  });

  it("closes after a successful save", async () => {
    const { answer, props } = renderAndSave();

    await act(async () => {
      answer.resolve();
    });

    expect(props.onSubmit).toHaveBeenCalledWith(
      [
        expect.objectContaining({
          guardianData: expect.objectContaining({ firstName: "Maxi" }),
        }),
      ],
      expect.any(Function),
    );
    expect(props.onClose).toHaveBeenCalledOnce();
  });

  it("does not close the panel that shows another guardian by then", async () => {
    const { answer, props, rerender } = renderAndSave();

    rerender(<GuardianFormModal {...props} initialData={erika} />);
    expect(await screen.findByDisplayValue("Erika")).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.resolve();
    });

    expect(screen.getByDisplayValue("Erika")).toBeInTheDocument();
    expect(props.onClose).not.toHaveBeenCalled();
  });
});
