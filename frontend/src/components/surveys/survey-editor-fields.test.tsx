import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { emptySurveyDraft } from "~/lib/survey-editor";
import { SurveyEditorFields } from "./survey-editor-fields";

it("verknüpft Teilnahme bis mit dem Datumsfeld", () => {
  render(
    <SurveyEditorFields
      draft={{ ...emptySurveyDraft(), audience: "staff" }}
      onChange={vi.fn()}
      errors={null}
      groups={[]}
      activities={[]}
      schoolClasses={[]}
      lookupLoadError={null}
    />,
  );

  expect(screen.getByText("Teilnahme bis")).toHaveAttribute(
    "for",
    "survey-closes-at",
  );
  expect(screen.getByLabelText("Teilnahme bis")).toHaveAttribute(
    "id",
    "survey-closes-at",
  );
});
