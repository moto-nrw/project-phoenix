import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LinkifiedText } from "./linkified-text";

describe("LinkifiedText", () => {
  it("renders plain text unchanged", () => {
    render(<LinkifiedText text="Nur Text ohne Link." />);
    expect(screen.getByText("Nur Text ohne Link.")).toBeInTheDocument();
  });

  it("turns an http(s) URL into a safe external link", () => {
    render(
      <LinkifiedText text="Infos unter https://schule.example.de/ausflug bitte lesen." />,
    );
    const link = screen.getByRole("link", {
      name: "https://schule.example.de/ausflug",
    });
    expect(link).toHaveAttribute("href", "https://schule.example.de/ausflug");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("keeps trailing sentence punctuation out of the link", () => {
    render(<LinkifiedText text="Siehe https://example.de/plan." />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "https://example.de/plan");
  });

  it("links multiple URLs and keeps surrounding text", () => {
    render(
      <LinkifiedText text="A https://a.example.de und https://b.example.de B" />,
    );
    expect(screen.getAllByRole("link")).toHaveLength(2);
  });

  it("never links non-http schemes", () => {
    render(<LinkifiedText text="Achtung javascript:alert(1) und ftp://x" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
