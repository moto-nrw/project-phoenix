import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import deMessages from "~/i18n/messages/de.generated";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
  ParentSectionSkeleton,
} from "./parent-page";

function withIntl(children: ReactNode) {
  return (
    <NextIntlClientProvider locale="de" messages={deMessages}>
      {children}
    </NextIntlClientProvider>
  );
}

describe("ParentPage", () => {
  const header = <ParentPageHeader title="Elternbriefe" />;

  it("keeps the head and replaces the content while loading", () => {
    render(
      withIntl(
        <ParentPage header={header} loading>
          <p>Inhalt</p>
        </ParentPage>,
      ),
    );

    expect(
      screen.getByRole("heading", { level: 1, name: "Elternbriefe" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveAccessibleName(
      "Inhalt wird geladen",
    );
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("renders the skeleton of the surface and its own announcement", () => {
    render(
      withIntl(
        <ParentPage
          header={header}
          loading={<p>Struktur</p>}
          loadingLabel="Termine werden geladen"
        />,
      ),
    );

    expect(screen.getByRole("status")).toHaveAccessibleName(
      "Termine werden geladen",
    );
    expect(screen.getByText("Struktur")).toBeInTheDocument();
  });

  it("shows the error instead of the content, with its retry", () => {
    const retry = vi.fn();
    render(
      withIntl(
        <ParentPage
          header={header}
          error={{
            message: "Die Elternbriefe konnten nicht geladen werden.",
            retry: { label: "Erneut versuchen", onClick: retry },
          }}
        >
          <p>Inhalt</p>
        </ParentPage>,
      ),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die Elternbriefe konnten nicht geladen werden.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    expect(retry).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("states an empty page in one sentence on a card", () => {
    render(
      withIntl(
        <ParentPage header={header} empty={{ title: "Noch keine Post" }}>
          <p>Inhalt</p>
        </ParentPage>,
      ),
    );

    expect(screen.getByText("Noch keine Post.")).toBeInTheDocument();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("keeps overlays mounted in every state", () => {
    render(
      withIntl(
        <ParentPage
          header={header}
          loading
          overlays={<div role="dialog" aria-label="Dialog" />}
        />,
      ),
    );

    expect(screen.getByRole("dialog", { name: "Dialog" })).toBeInTheDocument();
  });

  it("renders plain children without any translation provider", () => {
    render(
      <ParentPage>
        <p>Inhalt</p>
      </ParentPage>,
    );

    expect(screen.getByText("Inhalt")).toBeInTheDocument();
  });
});

describe("ParentPageHeader", () => {
  it("carries kicker, level-one title and back link", () => {
    render(
      withIntl(
        <ParentPageHeader
          kicker="Post"
          title="Elternbriefe"
          backHref="/parents"
          backLabel="Zurück"
        />,
      ),
    );

    expect(screen.getByText("Post")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Elternbriefe" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Zurück" })).toHaveAttribute(
      "href",
      "/parents",
    );
  });
});

describe("ParentPageSkeleton", () => {
  it.each([0, 2])("exposes one loading status with %s sections", (rows) => {
    render(withIntl(<ParentPageSkeleton rows={rows} />));

    expect(screen.getAllByRole("status", { hidden: true })).toHaveLength(1);
    expect(screen.getByRole("status")).toHaveTextContent("Inhalt wird geladen");
    expect(
      screen.queryAllByTestId("parent-page-section-skeleton"),
    ).toHaveLength(rows);
    for (const section of screen.queryAllByTestId(
      "parent-page-section-skeleton",
    )) {
      expect(section).toHaveAttribute("aria-hidden", "true");
    }
  });
});

describe("ParentSectionSkeleton", () => {
  it("keeps separate loading statuses for independent sections by default", () => {
    render(
      withIntl(
        <>
          <ParentSectionSkeleton />
          <ParentSectionSkeleton />
        </>,
      ),
    );

    expect(screen.getAllByRole("status")).toHaveLength(2);
  });

  // Beim ersten Öffnen eines Reiters steht nur der Platzhalter da. Ein
  // Screenreader muss hören, dass Inhalt kommt, statt nichts (#4994).
  it("sagt Screenreadern, dass Inhalt geladen wird", () => {
    render(withIntl(<ParentSectionSkeleton rows={2} />));

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Inhalt wird geladen");
    expect(status).toBe(screen.getByTestId("parent-page-section-skeleton"));
  });
});
