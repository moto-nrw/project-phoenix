import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  HelpEntry,
  ROLES_WITH_SCHOOL_STEP,
  type HelpEntryAnswers,
} from "./help-entry";

function renderEntry() {
  const onSubmit = vi.fn<(answers: HelpEntryAnswers) => void>();
  render(<HelpEntry onSubmit={onSubmit} />);
  return onSubmit;
}

describe("HelpEntry", () => {
  it("wears the same header as the guide", () => {
    renderEntry();

    const brandName = screen.getByText("moto");
    expect(brandName).toHaveClass("[font-family:var(--font-moto)]");
    expect(brandName.parentElement).toHaveTextContent("moto Hilfe");
    expect(
      document.querySelector('img[src*="moto_transparent"]'),
    ).not.toBeNull();
  });

  it("asks for the role before showing any guide", () => {
    renderEntry();

    expect(
      screen.getByRole("heading", { name: "Für wen ist die Anleitung?" }),
    ).toBeInTheDocument();
    for (const label of [
      "Betreuungskraft",
      "Leitung",
      "Elternteil",
      "Lehrkraft",
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("asks how the school works after an OGS role", () => {
    renderEntry();

    fireEvent.click(screen.getByText("Betreuungskraft"));

    expect(
      screen.getByRole("heading", { name: "Wie arbeitet Ihre OGS?" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Weiß ich nicht")).toHaveLength(3);
  });

  it("preselects no answer, so an open question stays visibly open", () => {
    renderEntry();

    fireEvent.click(screen.getByText("Betreuungskraft"));

    // "Weiss ich nicht" muss man waehlen. Vorausgewaehlt saehe die Frage
    // beantwortet aus, obwohl niemand geantwortet hat.
    const questions = screen.getAllByRole("group");
    expect(questions).toHaveLength(3);
    for (const question of questions) {
      const tiles = within(question).getAllByRole("button");
      expect(tiles).toHaveLength(3);
      for (const tile of tiles) {
        expect(tile).toHaveAttribute("aria-pressed", "false");
      }
    }
  });

  it("marks a selected school answer clearly", () => {
    renderEntry();

    fireEvent.click(screen.getByText("Betreuungskraft"));
    const nfcQuestion = screen.getByRole("group", {
      name: "Nutzt Ihre OGS ein NFC-Tablet?",
    });
    const yes = within(nfcQuestion).getByRole("button", { name: "Ja" });
    const no = within(nfcQuestion).getByRole("button", { name: "Nein" });

    fireEvent.click(yes);

    expect(yes).toHaveAttribute("aria-pressed", "true");
    expect(yes).toHaveAttribute("data-state", "selected");
    expect(no).toHaveAttribute("data-state", "unselected");
    expect(yes).toHaveClass("bg-moto-green-soft");
    expect(yes).not.toHaveClass("bg-moto-green/10");
    expect(yes.querySelector("svg")).toHaveClass("opacity-100");
    expect(no.querySelector("svg")).toHaveClass("opacity-0");
  });

  it("sends roles from the other portals straight to their guide", () => {
    const onSubmit = renderEntry();

    fireEvent.click(screen.getByText("Elternteil"));

    // Eltern arbeiten im Eltern-Portal. NFC, Gruppenmodell und
    // Anwesenheits-Modus aendern dort keinen Ablauf, also wird nicht gefragt.
    expect(onSubmit).toHaveBeenCalledWith({
      role: "parent",
      nfcEnabled: null,
      groupMode: "unknown",
      presenceMode: "unknown",
    });
    expect(
      screen.queryByRole("heading", { name: "Wie arbeitet Ihre OGS?" }),
    ).not.toBeInTheDocument();
  });

  it("passes the chosen school settings on", () => {
    const onSubmit = renderEntry();

    fireEvent.click(screen.getByText("Betreuungskraft"));
    fireEvent.click(screen.getByText("Nein"));
    fireEvent.click(screen.getByText("Offene Betreuung"));
    fireEvent.click(screen.getByText("Nur da oder nicht da"));
    fireEvent.click(screen.getByRole("button", { name: "Zur Anleitung" }));

    expect(onSubmit).toHaveBeenCalledWith({
      role: "caregiver",
      nfcEnabled: false,
      groupMode: "open_care",
      presenceMode: "binary",
    });
  });

  it("keeps every setting open when the school step is skipped", () => {
    const onSubmit = renderEntry();

    fireEvent.click(screen.getByText("Leitung"));
    fireEvent.click(screen.getByRole("button", { name: "Überspringen" }));

    // "Unbekannt" ist eine vollwertige Antwort: die Anleitung erklaert dann,
    // wann ein Ablauf ueberhaupt vorkommt, statt einen Modus zu erfinden.
    expect(onSubmit).toHaveBeenCalledWith({
      role: "lead",
      nfcEnabled: null,
      groupMode: "unknown",
      presenceMode: "unknown",
    });
  });

  it("lets a person correct the role", () => {
    const onSubmit = renderEntry();

    fireEvent.click(screen.getByText("Betreuungskraft"));
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));

    expect(
      screen.getByRole("heading", { name: "Für wen ist die Anleitung?" }),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("only asks the school step for roles inside the OGS app", () => {
    expect([...ROLES_WITH_SCHOOL_STEP]).toEqual(["caregiver", "lead"]);
  });
});
