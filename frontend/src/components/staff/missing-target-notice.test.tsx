import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MissingTargetDialogNotice } from "./missing-target-notice";

const mocks = vi.hoisted(() => ({
  hasTarget: null as boolean | null,
  args: [] as unknown[],
}));

vi.mock("~/lib/hooks/use-staff-has-target", () => ({
  useStaffHasTarget: (...args: unknown[]) => {
    mocks.args = args;
    return mocks.hasTarget;
  },
}));

function renderNotice(canSetTarget: boolean) {
  render(
    <MissingTargetDialogNotice
      staffId="7"
      staffName="Ben Brandt"
      date="2026-09-09"
      entryLabel="Die Krankmeldung"
      canSetTarget={canSetTarget}
    />,
  );
}

// #6887: „Krank melden“ und „Abwesenheit eintragen“ warnen, wenn die Person
// in diesem Monat kein Soll hat.
describe("MissingTargetDialogNotice", () => {
  beforeEach(() => {
    mocks.hasTarget = null;
  });

  it("warnt ohne Soll und nennt den Weg zum Arbeitszeitmodell", () => {
    mocks.hasTarget = false;
    renderNotice(true);

    expect(
      screen.getByText(/Für Ben Brandt ist kein Soll hinterlegt\./),
    ).toHaveTextContent(
      "Die Krankmeldung zählt deshalb 0 Stunden. Gilt später ein Soll ab diesem Tag, rechnet moto die Stunden nach. Das Soll legen Sie in der Personalakte unter „Arbeitszeitmodell“ fest.",
    );
    expect(mocks.args).toEqual(["7", "2026-09-09", true]);
  });

  it("nennt ohne Recht, wer das Soll einträgt", () => {
    mocks.hasTarget = false;
    renderNotice(false);

    expect(
      screen.getByText(/Für Ben Brandt ist kein Soll hinterlegt\./),
    ).toHaveTextContent("Das Soll trägt die Leitung ein.");
  });

  it.each([true, null])("bleibt still bei hasTarget %s", (value) => {
    mocks.hasTarget = value;
    const { container } = render(
      <MissingTargetDialogNotice
        staffId="7"
        staffName="Ben Brandt"
        date="2026-09-09"
        entryLabel="Die Krankmeldung"
        canSetTarget
      />,
    );

    expect(container.firstChild).toBeNull();
  });
});
