import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SectionCard } from "./section-card";

describe("SectionCard Kopfzeilen-Aktionen", () => {
  it("bricht Aktionen auf dem Telefon standardmäßig als eigene Zeile um", () => {
    render(
      <SectionCard
        title="Ablauf"
        actions={<button type="button">Öffnen</button>}
      >
        Inhalt
      </SectionCard>,
    );

    const wrapper = screen.getByRole("button", {
      name: "Öffnen",
    }).parentElement;
    expect(wrapper?.className).toContain("order-last");
    expect(wrapper?.className).toContain("w-full");
  });

  it("hält Aktionen mit inlineActions auf jeder Breite neben dem Titel", () => {
    render(
      <SectionCard
        title="Ablauf"
        inlineActions
        actions={<button type="button">Öffnen</button>}
      >
        Inhalt
      </SectionCard>,
    );

    const wrapper = screen.getByRole("button", {
      name: "Öffnen",
    }).parentElement;
    expect(wrapper?.className).toContain("shrink-0");
    expect(wrapper?.className).toContain("h-10");
    expect(wrapper?.className).not.toContain("order-last");
    expect(wrapper?.className).not.toContain("w-full");
  });
});
