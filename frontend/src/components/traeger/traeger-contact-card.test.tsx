import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

vi.mock("~/lib/traeger-api", () => ({ saveTraegerContact: vi.fn() }));

import { TraegerContactCard } from "./traeger-contact-card";

const SAVED = { name: "Petra Lindner", phone: "", email: "" };

function card(contact: typeof SAVED) {
  return (
    <ToastProvider>
      <TraegerContactCard
        contact={contact}
        canEdit
        onSaved={() => Promise.resolve()}
      />
    </ToastProvider>
  );
}

describe("TraegerContactCard", () => {
  it("keeps an open draft when the page reads the master data again", () => {
    const { rerender } = render(card(SAVED));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Jonas Weber" },
    });

    // Another change on the page reads the master data again: a new object
    // with the same stored contact.
    rerender(card({ ...SAVED }));

    expect(screen.getByLabelText("Name")).toHaveValue("Jonas Weber");
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });

  it("takes a new stored contact while nothing is being edited", () => {
    const { rerender } = render(card(SAVED));

    rerender(card({ ...SAVED, phone: "+49 5402 1234-0" }));

    expect(screen.getByLabelText("Telefon")).toHaveValue("+49 5402 1234-0");
  });
});
