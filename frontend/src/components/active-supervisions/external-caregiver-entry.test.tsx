import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { createExternal } = vi.hoisted(() => ({ createExternal: vi.fn() }));

vi.mock("~/lib/staff-api", () => ({
  staffService: { createExternal },
}));

import { ExternalCaregiverEntry } from "./external-caregiver-entry";

function openEntry(): void {
  fireEvent.click(
    screen.getByRole("button", { name: "Externe Person eintragen" }),
  );
}

function fillName(first: string, last: string, organization = ""): void {
  fireEvent.change(screen.getByLabelText("Vorname"), {
    target: { value: first },
  });
  fireEvent.change(screen.getByLabelText("Nachname"), {
    target: { value: last },
  });
  fireEvent.change(screen.getByLabelText("Organisation (freiwillig)"), {
    target: { value: organization },
  });
}

describe("ExternalCaregiverEntry", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createExternal.mockResolvedValue({
      id: "88",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
      externalOrganization: "Musikschule",
    });
  });

  it("records an external person and hands the new entry back", async () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry existing={[]} onAdded={onAdded} />);

    openEntry();
    fillName(" Lea ", "Gast", "Musikschule");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith({
        kind: "created",
        staff: expect.objectContaining({ id: "88" }),
      }),
    );
    expect(createExternal).toHaveBeenCalledWith({
      firstName: "Lea",
      lastName: "Gast",
      organization: "Musikschule",
    });
    expect(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    ).toBeInTheDocument();
  });

  it("asks for both names before sending anything", () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry existing={[]} onAdded={onAdded} />);

    openEntry();
    fillName("Lea", "  ");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(
      screen.getByText("Bitte geben Sie Vor- und Nachnamen ein."),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte Nachnamen eingeben.")).toBeInTheDocument();
    expect(createExternal).not.toHaveBeenCalled();
    expect(onAdded).not.toHaveBeenCalled();
  });

  it("selects a person who is already in the list instead of adding a twin", () => {
    const onAdded = vi.fn();
    render(
      <ExternalCaregiverEntry
        existing={[{ id: "5", fullName: "Lea Gast", isExternal: true }]}
        onAdded={onAdded}
      />,
    );

    openEntry();
    fillName("  lea ", "GAST");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(onAdded).toHaveBeenCalledWith({ kind: "existing", id: "5" });
    expect(createExternal).not.toHaveBeenCalled();
  });

  it("does not select an internal person with the same name", async () => {
    const onAdded = vi.fn();
    render(
      <ExternalCaregiverEntry
        existing={[{ id: "5", fullName: "Lea Gast", isExternal: false }]}
        onAdded={onAdded}
      />,
    );

    openEntry();
    fillName("Lea", "Gast");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() => expect(createExternal).toHaveBeenCalledOnce());
    expect(onAdded).toHaveBeenCalledWith({
      kind: "created",
      staff: expect.objectContaining({ id: "88" }),
    });
  });

  it("keeps Enter and Escape inside the panel when the dialog is a form", async () => {
    const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
      event.preventDefault(),
    );
    const onAdded = vi.fn();
    render(
      <form onSubmit={onSubmit}>
        <ExternalCaregiverEntry existing={[]} onAdded={onAdded} />
      </form>,
    );

    openEntry();
    fillName("Lea", "Gast");
    fireEvent.keyDown(screen.getByLabelText("Nachname"), { key: "Enter" });
    await waitFor(() => expect(onAdded).toHaveBeenCalledOnce());
    expect(onSubmit).not.toHaveBeenCalled();

    openEntry();
    fireEvent.keyDown(screen.getByLabelText("Vorname"), { key: "Escape" });
    expect(screen.queryByLabelText("Vorname")).not.toBeInTheDocument();
  });
});
