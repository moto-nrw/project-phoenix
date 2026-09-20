import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import NfcQuickstartPage from "./page";

// next/link -> plain anchor, next/image -> plain img (the page only needs the
// rendered href/alt, not the Next.js runtime).
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

vi.mock("next/image", () => ({
  default: ({ alt, src }: { alt: string; src: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={alt} src={src} />
  ),
}));

describe("NfcQuickstartPage", () => {
  it("verweist auf die NFC-Anleitungen der Hilfe", () => {
    render(<NfcQuickstartPage />);

    const link = screen.getByRole("link", { name: /NFC-Anleitungen öffnen/i });
    // `nfc_enabled` muss mit: ohne den Schalter tragen die Karten der
    // Kategorie nur den Platzhalter statt ihres Kurztexts. Dass die Schule
    // NFC nutzt, ist beim Blatt aus dem Versandkarton bekannt.
    expect(link).toHaveAttribute(
      "href",
      "/help/gruppe/nfc?role=caregiver&nfc_enabled=true",
    );
  });

  it("nennt den Weg zur Hilfe auch für den Ausdruck", () => {
    render(<NfcQuickstartPage />);

    expect(
      screen.getByText(/unten links auf\s+Hilfe klicken/i),
    ).toBeInTheDocument();
  });

  it("zeigt die drei Erste-Schritte-Karten", () => {
    render(<NfcQuickstartPage />);

    expect(
      screen.getByRole("heading", { name: "Tablet montieren und einschalten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Mit Geräte-PIN anmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Armbänder zuweisen" }),
    ).toBeInTheDocument();
  });
});
