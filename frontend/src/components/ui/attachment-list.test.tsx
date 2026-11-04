import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AttachmentList } from "./attachment-list";

// #5330: a file the browser shows opens in a new tab, so its row must not
// carry a download arrow; only a real download does.
describe("AttachmentList", () => {
  it("marks a file the browser shows as opening, not as a download", () => {
    const { container } = render(
      <AttachmentList
        attachments={[
          {
            id: "1",
            filename: "nachweis.pdf",
            size_bytes: 1024,
            content_type: "application/pdf",
          },
        ]}
        downloadUrl={(id) => `/files/${id}`}
        openLabel="In neuem Tab öffnen"
      />,
    );

    // Anker: die Zeile ist gerendert und zeigt das Öffnen-Symbol.
    const link = screen.getByRole("link", { name: "nachweis.pdf" });
    expect(link).toHaveAttribute("href", "/files/1?inline=1");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("title", "In neuem Tab öffnen");
    expect(container.querySelector(".lucide-external-link")).not.toBeNull();
    expect(container.querySelector(".lucide-download")).toBeNull();
  });

  it("keeps the download arrow for a file the browser downloads", () => {
    const { container } = render(
      <AttachmentList
        attachments={[
          {
            id: "2",
            filename: "liste.xlsx",
            size_bytes: 2048,
            content_type:
              "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          },
        ]}
        downloadUrl={(id) => `/files/${id}`}
      />,
    );

    // Anker: die Zeile ist gerendert und zeigt den Download-Pfeil.
    const link = screen.getByRole("link", { name: "liste.xlsx" });
    expect(link).toHaveAttribute("href", "/files/2");
    expect(link).not.toHaveAttribute("target");
    expect(container.querySelector(".lucide-download")).not.toBeNull();
    expect(container.querySelector(".lucide-external-link")).toBeNull();
  });
});
