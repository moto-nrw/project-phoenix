import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";

// The link to the letterhead page needs no portal host here.
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
const { mockSave } = vi.hoisted(() => ({ mockSave: vi.fn() }));
vi.mock("~/lib/traeger-api", () => ({
  saveTraegerWithdrawalContact: mockSave,
}));

import { TraegerWithdrawalContactCard } from "./traeger-withdrawal-contact-card";

const CONTACT = {
  name: "Petra Lindner",
  phone: "0211 123 45",
  email: "p.lindner@traeger.example",
};
const LETTERHEAD = "Ganztag im Quartier e.V.\nAm Markt 4\n40210 Düsseldorf";

function card(
  withdrawal = { phone: "", email: "" },
  contact = CONTACT,
  canEdit = true,
) {
  return (
    <ToastProvider>
      <TraegerWithdrawalContactCard
        withdrawal={withdrawal}
        contact={contact}
        letterheadText={LETTERHEAD}
        canEdit={canEdit}
        onSaved={() => Promise.resolve()}
      />
    </ToastProvider>
  );
}

function preview() {
  return within(
    screen.getByRole("figure", { name: "Vorschau der Widerrufsbelehrung" }),
  );
}

describe("TraegerWithdrawalContactCard", () => {
  it("shows the contact person's phone and e-mail in the notice while the fields are empty", () => {
    render(card());

    expect(screen.getByLabelText("Telefon für Widerrufe")).toHaveAttribute(
      "placeholder",
      "0211 123 45",
    );
    expect(
      screen.getByText(
        "Leer lassen: Dann steht hier 0211 123 45 vom Ansprechpartner.",
      ),
    ).toBeInTheDocument();
    expect(
      preview().getByText(
        "Ganztag im Quartier e.V., Am Markt 4, 40210 Düsseldorf, Telefon: 0211 123 45, E-Mail: p.lindner@traeger.example",
      ),
    ).toBeInTheDocument();
    expect(preview().getAllByText("Vom Ansprechpartner")).toHaveLength(2);
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
  });

  it("updates the preview as the office types its own e-mail", () => {
    render(card());
    fireEvent.change(screen.getByLabelText("E-Mail für Widerrufe"), {
      target: { value: "widerruf@traeger.example" },
    });

    expect(
      preview().getByText(
        /Telefon: 0211 123 45, E-Mail: widerruf@traeger\.example$/,
      ),
    ).toBeInTheDocument();
    expect(preview().getByText("Eigene Angabe")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });

  it("warns when the notice would miss a phone or an e-mail", () => {
    render(card({ phone: "", email: "" }, { name: "", phone: "", email: "" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Es fehlt: Telefonnummer und E-Mail-Adresse.",
    );
    expect(preview().getAllByText("Fehlt")).toHaveLength(2);
  });

  it("saves and shows the values as the backend keeps them", async () => {
    mockSave.mockResolvedValue({
      phone: "",
      email: "widerruf@traeger.example",
    });
    render(card());
    fireEvent.change(screen.getByLabelText("E-Mail für Widerrufe"), {
      target: { value: " Widerruf@Traeger.Example " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(screen.getByLabelText("E-Mail für Widerrufe")).toHaveValue(
        "widerruf@traeger.example",
      ),
    );
    expect(mockSave).toHaveBeenCalledWith({
      phone: "",
      // An e-mail field drops the spaces itself; the backend lowers the case.
      email: "Widerruf@Traeger.Example",
    });
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
  });

  it("shows the fields read-only without the right to change them", () => {
    render(card({ phone: "0800 111", email: "" }, CONTACT, false));

    expect(screen.getByLabelText("Telefon für Widerrufe")).toBeDisabled();
    expect(preview().getByText("Eigene Angabe")).toBeInTheDocument();
  });

  // #6652: the save holds once the write is through; a failed reload of the
  // page is a load error in the card that says the change is saved.
  it("keeps the saved values when the page reload fails", async () => {
    mockSave.mockResolvedValue({
      phone: "",
      email: "widerruf@traeger.example",
    });
    const onSaved = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(undefined);
    render(
      <ToastProvider>
        <TraegerWithdrawalContactCard
          withdrawal={{ phone: "", email: "" }}
          contact={CONTACT}
          letterheadText={LETTERHEAD}
          canEdit
          onSaved={onSaved}
        />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText("E-Mail für Widerrufe"), {
      target: { value: "widerruf@traeger.example" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige des Kontakts für Widerrufe")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(2));
    expect(mockSave).toHaveBeenCalledTimes(1);
  });
});
