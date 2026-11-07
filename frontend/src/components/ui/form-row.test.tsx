import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "./button";
import { FormRow } from "./form-row";
import { Input } from "./input";

describe("FormRow", () => {
  it("keeps the field and its button in reading order", () => {
    render(
      <FormRow>
        <Input name="code" label="Code" />
        <Button type="button">Prüfen</Button>
      </FormRow>,
    );

    const field = screen.getByRole("textbox", { name: "Code" });
    const button = screen.getByRole("button", { name: "Prüfen" });
    expect(
      field.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});
