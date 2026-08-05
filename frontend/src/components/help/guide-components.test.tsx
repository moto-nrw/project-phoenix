import { render, screen, within } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Activity } from "lucide-react";
import { EntryPointCard, GuideShell } from "./guide-components";
import { GuidePdfButton } from "./guide-pdf-button";
import type { GuideChapter } from "./guide-data";

// next/link -> plain anchor (forwarding the props the components rely on).
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
    ...rest
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className} {...rest}>
      {children}
    </a>
  ),
}));

function makeChapters(): GuideChapter[] {
  return [
    {
      id: "kap-1",
      title: "Erstes Kapitel",
      description: "Beschreibung eins",
      icon: Activity,
      tone: "green",
      steps: [
        {
          id: "s1",
          title: "Schritt A",
          summary: "Bild-Schritt",
          image: "/help/screens/a.webp",
          screenshot: "Bild A",
        },
        {
          id: "s2",
          title: "Schritt B",
          summary: "Ohne Bild",
          screenshot: "Bild B",
        },
      ],
    },
    {
      id: "kap-2",
      title: "Zweites Kapitel",
      description: "Beschreibung zwei",
      icon: Activity,
      tone: "blue",
      steps: [
        {
          id: "s3",
          title: "Schritt C",
          summary: "Galerie-Schritt",
          screenshot: "ignoriert",
          gallery: [
            { image: "/help/screens/g1.webp", caption: "Galeriebild 1" },
            { image: "/help/screens/g2.webp", caption: "Galeriebild 2" },
          ],
        },
      ],
    },
  ];
}

function makeFourChapters(): GuideChapter[] {
  return [
    ...makeChapters(),
    {
      id: "kap-3",
      title: "Drittes Kapitel",
      description: "Beschreibung drei",
      icon: Activity,
      tone: "orange",
      steps: [
        {
          id: "s4",
          title: "Schritt D",
          summary: "Weiterer Schritt",
          screenshot: "Bild D",
        },
      ],
    },
    {
      id: "kap-4",
      title: "Viertes Kapitel",
      description: "Beschreibung vier",
      icon: Activity,
      tone: "purple",
      steps: [
        {
          id: "s5",
          title: "Schritt E",
          summary: "Letzter Schritt",
          screenshot: "Bild E",
        },
      ],
    },
  ];
}

describe("EntryPointCard", () => {
  it("renders title, body, every bullet point and the href", () => {
    const { container } = render(
      <EntryPointCard
        href="/help/setup"
        title="Ersteinrichtung"
        body="Kurzbeschreibung"
        icon={Activity}
        points={["Punkt eins", "Punkt zwei"]}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Ersteinrichtung" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Kurzbeschreibung")).toBeInTheDocument();
    expect(screen.getByText("Punkt eins")).toBeInTheDocument();
    expect(screen.getByText("Punkt zwei")).toBeInTheDocument();
    expect(container.querySelector("a")).toHaveAttribute("href", "/help/setup");
    expect(screen.getByTestId("entry-point-icon").className).not.toMatch(
      /\b(?:bg-|rounded)/,
    );
  });

  it("does not crash when two bullet points are identical (stable keys)", () => {
    render(
      <EntryPointCard
        href="/help/x"
        title="T"
        body="B"
        icon={Activity}
        points={["dup", "dup"]}
      />,
    );
    expect(screen.getAllByText("dup")).toHaveLength(2);
  });
});

describe("GuideShell", () => {
  it("renders the header, eyebrow/title/description and the PDF button", () => {
    render(
      <GuideShell
        eyebrow="Ersteinrichtung"
        title="Schritt für Schritt"
        description="Eine Beschreibung"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    expect(
      screen.queryByRole("link", { name: /moto\s+Anleitung/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Zur Übersicht").closest("a")).toHaveAttribute(
      "href",
      "/help",
    );
    expect(screen.getAllByText("Ersteinrichtung")).toHaveLength(2);
    expect(
      screen.getAllByRole("heading", {
        name: "Schritt für Schritt",
        level: 1,
      }),
    ).toHaveLength(2);
    expect(screen.getAllByText("Eine Beschreibung")).toHaveLength(2);
    expect(screen.getByText("Setup Guide")).toBeInTheDocument();

    // PDF button -> setup.pdf for the ersteinrichtung activePath.
    const pdf = screen.getByText("PDF herunterladen").closest("a");
    expect(pdf).toHaveAttribute("href", "/help/pdfs/setup.pdf");
    expect(pdf).toHaveAttribute(
      "download",
      "moto Anleitung - Ersteinrichtung.pdf",
    );
  });

  it("renders chapter headings and the on-this-page table of contents", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Erstes Kapitel" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Zweites Kapitel" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Auf dieser Seite")).toBeInTheDocument();
    // Continuous chapter numbering in the TOC.
    expect(screen.getByText(/1\.\s*Erstes Kapitel/)).toBeInTheDocument();
    expect(screen.getByText(/2\.\s*Zweites Kapitel/)).toBeInTheDocument();
  });

  it("numbers step badges continuously across chapters when numbered=true", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    // Chapter 1 has 2 steps, chapter 2 starts at badge "3" (not "1").
    const article = document.getElementById("s3");
    expect(article).not.toBeNull();
    expect(within(article as HTMLElement).getByText("3")).toBeInTheDocument();
  });

  it.each(["ersteinrichtung", "funktionen", "nfc"] as const)(
    "does not force PDF page breaks before chapters for %s",
    (activePath) => {
      render(
        <GuideShell
          eyebrow="E"
          title="T"
          description="D"
          chapters={makeFourChapters()}
          activePath={activePath}
          numbered
        />,
      );

      for (const id of ["kap-1", "kap-2", "kap-3", "kap-4"]) {
        expect(document.getElementById(id)).not.toHaveClass(
          "print:[break-before:page]",
          "print:[page-break-before:always]",
        );
      }
    },
  );

  it("keeps chapter headers together with the following PDF content", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    const chapter = document.getElementById("kap-1");
    const chapterElement = chapter as HTMLElement;
    const heading = within(chapterElement).getByRole("heading", {
      name: "Erstes Kapitel",
    });

    expect(heading).toBeInTheDocument();
    expect(chapterElement.firstElementChild).toHaveClass(
      "print:[break-inside:avoid]",
      "print:[break-after:avoid]",
    );
  });

  it("renders a step image with the caption as alt text, and nothing for image-less steps", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    const img = screen.getByAltText("Bild A");
    expect(img.getAttribute("src")).toContain("%2Fhelp%2Fscreens%2Fa.webp");
    // The image-less step renders no <img> (no placeholder).
    expect(screen.queryByAltText("Bild B")).not.toBeInTheDocument();
  });

  it("renders every gallery image with its caption", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    expect(screen.getByAltText("Galeriebild 1").getAttribute("src")).toContain(
      "%2Fhelp%2Fscreens%2Fg1.webp",
    );
    expect(screen.getByAltText("Galeriebild 2")).toBeInTheDocument();
    // Captions render below each figure.
    expect(screen.getByText("Galeriebild 1")).toBeInTheDocument();
  });

  it("renders ordered sub-steps, a checklist and a callout", () => {
    const chapters: GuideChapter[] = [
      {
        id: "kap",
        title: "Kapitel",
        description: "d",
        icon: Activity,
        tone: "orange",
        steps: [
          {
            id: "step",
            title: "Komplexer Schritt",
            summary: "Mit allem",
            steps: ["Erste Aktion", "Zweite Aktion"],
            checklist: ["Prüfpunkt eins"],
            callout: { title: "Wichtiger Hinweis", body: "Hinweistext" },
            screenshot: "x",
          },
          {
            // numbered=false + an icon -> the StepCard renders the icon in place
            // of a number badge (the `badge ?? (Icon ? <Icon/> : null)` branch).
            id: "icon-step",
            title: "Icon-Schritt",
            summary: "Zeigt ein Icon statt einer Nummer",
            icon: Activity,
            screenshot: "y",
          },
        ],
      },
    ];

    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={chapters}
        activePath="ersteinrichtung"
        numbered={false}
      />,
    );

    expect(screen.getByText("Erste Aktion")).toBeInTheDocument();
    expect(screen.getByText("Zweite Aktion")).toBeInTheDocument();
    expect(screen.getByText("Checkliste")).toBeInTheDocument();
    expect(screen.getByText("Prüfpunkt eins")).toBeInTheDocument();
    expect(screen.getByText("Wichtiger Hinweis")).toBeInTheDocument();
    expect(screen.getByText("Hinweistext")).toBeInTheDocument();
    expect(screen.getByText("Icon-Schritt")).toBeInTheDocument();
  });

  it("renders inline `code` spans from backtick markup in the note", () => {
    const { container } = render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
        note="Pflegen Sie alles unter `Datenverwaltung` zentral."
      />,
    );

    const code = container.querySelector("code");
    expect(code).not.toBeNull();
    expect(code).toHaveTextContent("Datenverwaltung");
  });

  it("renders the activePath-specific next link", () => {
    render(
      <GuideShell
        eyebrow="E"
        title="T"
        description="D"
        chapters={makeChapters()}
        activePath="ersteinrichtung"
        numbered
      />,
    );

    // ersteinrichtung -> links forward to the features guide.
    const next = screen.getByText("Die App entdecken").closest("a");
    expect(next).toHaveAttribute("href", "/help/features");
  });
});

describe("GuidePdfButton", () => {
  it("renders a download anchor with the given href and filename", () => {
    render(
      <GuidePdfButton href="/help/pdfs/nfc.pdf" download="NFC Anleitung.pdf" />,
    );

    const link = screen.getByText("PDF herunterladen").closest("a");
    expect(link).toHaveAttribute("href", "/help/pdfs/nfc.pdf");
    expect(link).toHaveAttribute("download", "NFC Anleitung.pdf");
  });
});
