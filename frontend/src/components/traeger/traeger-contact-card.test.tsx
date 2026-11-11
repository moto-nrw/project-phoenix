import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";

const { mockSave } = vi.hoisted(() => ({ mockSave: vi.fn() }));
vi.mock("~/lib/traeger-api", () => ({ saveTraegerContact: mockSave }));

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

  it("shows the contact as the backend keeps it after saving", async () => {
    mockSave.mockResolvedValue({
      name: "Petra Lindner",
      phone: "",
      email: "buero@traeger.example",
    });
    render(card(SAVED));
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: " Buero@Traeger.Example " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(screen.getByLabelText("E-Mail-Adresse")).toHaveValue(
        "buero@traeger.example",
      ),
    );
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Petra Lindner");
  });

  // #6652: the save holds once the write is through; a failed reload of the
  // page is a load error in the card that says the change is saved.
  it("keeps the saved values when the page reload fails", async () => {
    mockSave.mockResolvedValue({
      name: "Petra Lindner",
      phone: "",
      email: "buero@traeger.example",
    });
    const onSaved = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(undefined);
    render(
      <ToastProvider>
        <TraegerContactCard contact={SAVED} canEdit onSaved={onSaved} />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "buero@traeger.example" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige des Ansprechpartners")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(2));
    expect(mockSave).toHaveBeenCalledTimes(1);
  });
});
