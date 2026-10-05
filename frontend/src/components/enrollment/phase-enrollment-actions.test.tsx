import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  createLateInvite: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return { ...actual, createLateInvite: mocks.createLateInvite };
});

import { LateInviteModal } from "./phase-enrollment-actions";
import { ApiError } from "~/lib/api-error";
import type { Phase } from "~/lib/enrollment-phase-api";
import { catalogText } from "~/test/error-catalog-text";

const phase = { id: "5", name: "Schuljahr 2026/27" } as Phase;

describe("LateInviteModal (#2515)", () => {
  beforeEach(() => {
    mocks.createLateInvite.mockReset();
  });

  it("keeps the dialog open and marks the e-mail the server refused", async () => {
    mocks.createLateInvite.mockRejectedValue(
      new ApiError("invalid email", 400, {
        code: "general.input",
        errors: [{ field: "guardian_email", reason: "invalid" }],
      }),
    );
    render(
      <LateInviteModal
        isOpen
        onClose={vi.fn()}
        phase={phase}
        phaseUrl="https://demo.example.test/anmeldung/5"
      />,
    );

    fireEvent.change(
      screen.getByLabelText("E-Mail der erziehungsberechtigten Person"),
      { target: { value: "eltern@example.test" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Link erstellen" }));
    expect(
      await screen.findByText(catalogText("general.input", "die Einladung")),
    ).toBeVisible();
    await waitFor(() =>
      expect(
        screen.getByLabelText("E-Mail der erziehungsberechtigten Person"),
      ).toHaveAttribute("aria-invalid", "true"),
    );
    expect(screen.queryByText("Link wurde erstellt.")).not.toBeInTheDocument();
  });

  it("names a missing phase link before sending", async () => {
    render(
      <LateInviteModal isOpen onClose={vi.fn()} phase={phase} phaseUrl="" />,
    );

    fireEvent.change(
      screen.getByLabelText("E-Mail der erziehungsberechtigten Person"),
      { target: { value: "eltern@example.test" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Link erstellen" }));

    expect(
      await screen.findByText(
        "Der Link zur Anmeldephase fehlt. Bitte laden Sie die Seite neu.",
      ),
    ).toBeVisible();
    expect(mocks.createLateInvite).not.toHaveBeenCalled();
  });
});
