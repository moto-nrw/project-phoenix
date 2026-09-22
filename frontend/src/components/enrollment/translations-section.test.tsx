import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import {
  TranslationsSection,
  type TranslationTarget,
} from "./translations-section";

const label: TranslationTarget = {
  id: "field|0|label",
  caption: "Frage",
  german: "Hat Ihr Kind Allergien?",
  attr: "label",
  translations: {
    ru: { label: { text: "Есть ли аллергии?", source: "Allergien?" } },
    en: {
      label: {
        text: "Does your child have allergies?",
        source: "Hat Ihr Kind Allergien?",
      },
    },
  },
};

describe("TranslationsSection", () => {
  it("says when the object has no own texts yet", () => {
    render(<TranslationsSection targets={[]} onChange={vi.fn()} />);
    expect(
      screen.getByText("Hier gibt es noch keine eigenen Texte zum Übersetzen."),
    ).toBeInTheDocument();
  });

  it("shows the German text next to the translation of the chosen language", () => {
    render(<TranslationsSection targets={[label]} onChange={vi.fn()} />);

    expect(screen.getByText("Hat Ihr Kind Allergien?")).toBeInTheDocument();
    expect(screen.getByLabelText("Übersetzung")).toHaveValue(
      "Does your child have allergies?",
    );
    expect(screen.getByText("Fertig")).toBeInTheDocument();
    expect(screen.getByText("1 von 1 übersetzt")).toBeInTheDocument();
  });

  it("flags a translation whose German text changed and lets the school confirm it", () => {
    const onChange = vi.fn();
    render(<TranslationsSection targets={[label]} onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: "Русский" }));

    expect(screen.getByText("Bitte prüfen")).toBeInTheDocument();
    expect(screen.getByText("0 von 1 übersetzt")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Passt noch" }));

    expect(onChange).toHaveBeenCalledWith(
      label,
      expect.objectContaining({
        ru: {
          label: {
            text: "Есть ли аллергии?",
            source: "Hat Ihr Kind Allergien?",
          },
        },
      }),
    );
  });

  it("records a typed translation against the current German text", () => {
    const onChange = vi.fn();
    render(
      <TranslationsSection
        targets={[{ ...label, translations: undefined }]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Polski" }));
    const row = screen.getByRole("listitem");
    expect(within(row).getByText("Fehlt")).toBeInTheDocument();
    fireEvent.change(within(row).getByLabelText("Übersetzung"), {
      target: { value: "Czy dziecko ma alergie?" },
    });

    expect(onChange).toHaveBeenCalledWith(expect.anything(), {
      pl: {
        label: {
          text: "Czy dziecko ma alergie?",
          source: "Hat Ihr Kind Allergien?",
        },
      },
    });
  });
});
