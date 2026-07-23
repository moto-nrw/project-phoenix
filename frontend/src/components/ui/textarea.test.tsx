import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Textarea } from "./textarea";

describe("Textarea label association", () => {
  it("uses an explicit id for the built-in label", () => {
    render(<Textarea id="answer" label="Antwort" />);

    expect(screen.getByLabelText("Antwort")).toHaveAttribute("id", "answer");
  });

  it("falls back to name when id is omitted", () => {
    render(<Textarea name="reason" label="Begründung" />);

    expect(screen.getByLabelText("Begründung")).toHaveAttribute("id", "reason");
  });

  it("keeps name for form submission when id differs", () => {
    render(<Textarea id="answer" name="response" label="Antwort" />);

    expect(screen.getByLabelText("Antwort")).toHaveAttribute(
      "name",
      "response",
    );
  });
});
