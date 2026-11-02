import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ChatBubble } from "./chat-bubble";

describe("ChatBubble in der Eltern-App", () => {
  it("zeigt eine OGS-Nachricht mit Absender sowie Datum und Uhrzeit", () => {
    render(
      <ChatBubble
        body="Bitte denken Sie an die Trinkflasche."
        own={false}
        senderName="OGS-Team der Demo School"
        createdAt="2026-08-16T06:50:00Z"
        tone="parent"
      />,
    );

    expect(screen.getByText("OGS-Team der Demo School")).toBeInTheDocument();
    expect(screen.getByText("16.08., 08:50")).toBeInTheDocument();
  });

  it("blendet den redundanten Teamnamen in einem kompakten Verlauf aus", () => {
    render(
      <ChatBubble
        body="Bitte denken Sie an die Trinkflasche."
        own={false}
        senderName="OGS-Team der Demo School"
        createdAt="2026-08-16T06:50:00Z"
        tone="parent"
        showSenderName={false}
      />,
    );

    expect(
      screen.queryByText("OGS-Team der Demo School"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("16.08., 08:50")).toBeInTheDocument();
  });

  it("zeigt eine eigene gesendete Nachricht mit zwei grauen Haken", () => {
    render(
      <ChatBubble
        body="Danke, mache ich."
        own
        senderName="Karin Klein"
        createdAt="2026-08-16T06:51:00Z"
        tone="parent"
        deliveryStatus="sent"
        deliveryStatusLabel="Gesendet"
        showOwnSenderName={false}
      />,
    );

    expect(screen.getByLabelText("Gesendet")).toBeInTheDocument();
    expect(screen.queryByText("Karin Klein")).not.toBeInTheDocument();
  });

  it("zeigt eine von der OGS gelesene Nachricht mit zwei Haken", () => {
    render(
      <ChatBubble
        body="Danke, mache ich."
        own
        senderName="Karin Klein"
        createdAt="2026-08-16T06:51:00Z"
        tone="parent"
        deliveryStatus="read"
        deliveryStatusLabel="Von der OGS gelesen"
        showOwnSenderName={false}
      />,
    );

    expect(screen.getByLabelText("Von der OGS gelesen")).toBeInTheDocument();
  });
});

// #5011: eine eigene Nachricht auf dem Weg oder nicht gesendet.
describe("ChatBubble beim Senden", () => {
  it("zeigt im Team-Chat den Zustand statt Name und Uhrzeit", () => {
    render(
      <ChatBubble
        body="Bin unterwegs"
        own
        senderName="Anna Beispiel"
        createdAt="2026-08-16T06:50:00Z"
        pending="pending"
      />,
    );

    // Anker: die Blase steht mit ihrem Zustand da.
    expect(screen.getByText("Bin unterwegs")).toBeInTheDocument();
    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
    expect(screen.queryByText(/Anna Beispiel/)).not.toBeInTheDocument();
    expect(screen.queryByText(/08:50/)).not.toBeInTheDocument();
  });

  it("zeigt in der Eltern-App den übergebenen Text ohne Haken", () => {
    render(
      <ChatBubble
        body="Hallo OGS"
        own
        senderName="Sabine Schneider"
        createdAt="2026-08-16T06:50:00Z"
        tone="parent"
        pending="failed"
        pendingLabel="Nicht gesendet"
        deliveryStatus="sent"
        deliveryStatusLabel="Gesendet"
      />,
    );

    // Anker: der Fehlzustand steht an der Blase.
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(
      screen.queryByRole("img", { name: "Gesendet" }),
    ).not.toBeInTheDocument();
  });
});
