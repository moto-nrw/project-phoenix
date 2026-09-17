import { render, screen } from "@testing-library/react";
import Link from "next/link";
import { describe, expect, it } from "vitest";
import { renderThreadFrame } from "./page";

describe("renderThreadFrame", () => {
  it("keeps the back navigation when the team chat is disabled", () => {
    render(
      renderThreadFrame({
        title: "Unterhaltung",
        roleLabel: null,
        stats: "",
        statsLoading: false,
        state: "disabled",
        empty: {
          icon: null,
          title: "Der Team-Chat ist ausgeschaltet",
          description: "Der Team-Chat ist nicht verfügbar.",
        },
        errorMessage: "",
        loading: false,
        backNav: <Link href="/team-chat">Zurück zum Team-Chat</Link>,
        containerRef: { current: null },
        body: null,
      }),
    );

    expect(
      screen.getByRole("link", { name: "Zurück zum Team-Chat" }),
    ).toHaveAttribute("href", "/team-chat");
  });

  it("opts the chat card out of the page grow rule so only the list scrolls", () => {
    render(
      renderThreadFrame({
        title: "Unterhaltung",
        roleLabel: null,
        stats: "",
        statsLoading: false,
        state: "ready",
        empty: { icon: null, title: "", description: "" },
        errorMessage: "",
        loading: false,
        backNav: null,
        containerRef: { current: null },
        body: <p>Verlauf</p>,
      }),
    );

    // Ohne die Klasse wächst die Karte auf die Höhe des ganzen Verlaufs und
    // die feste Hülle schneidet das Eingabefeld ab (#3328).
    expect(screen.getByText("Verlauf").closest("section")).toHaveClass(
      "moto-scroll-surface",
    );
  });
});
