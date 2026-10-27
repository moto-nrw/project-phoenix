import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import deMessages from "~/i18n/messages/de.json";
import { ParentPageSkeleton, ParentSectionSkeleton } from "./parent-page";

function withIntl(children: ReactNode) {
  return (
    <NextIntlClientProvider locale="de" messages={deMessages}>
      {children}
    </NextIntlClientProvider>
  );
}

describe("ParentPageSkeleton", () => {
  it("reserves the header and section internals instead of using solid blocks", () => {
    render(withIntl(<ParentPageSkeleton rows={2} />));

    expect(screen.getByTestId("parent-page-header-skeleton")).toHaveClass(
      "moto-content-surface",
    );
    expect(screen.getAllByTestId("parent-page-section-skeleton")).toHaveLength(
      2,
    );
    expect(
      screen.getAllByTestId("parent-page-section-row-skeleton"),
    ).toHaveLength(6);
  });
});

describe("ParentSectionSkeleton", () => {
  // Beim ersten Öffnen eines Reiters steht nur der Platzhalter da. Ein
  // Screenreader muss hören, dass Inhalt kommt, statt nichts (#4994).
  it("sagt Screenreadern, dass Inhalt geladen wird", () => {
    render(withIntl(<ParentSectionSkeleton rows={2} />));

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Inhalt wird geladen");
    expect(status).toBe(screen.getByTestId("parent-page-section-skeleton"));
    expect(
      screen
        .getAllByTestId("parent-page-section-row-skeleton")[0]
        ?.closest('[aria-hidden="true"]'),
    ).toBeInTheDocument();
  });
});
