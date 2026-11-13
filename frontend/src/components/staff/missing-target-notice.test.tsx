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
      screen.getByText(
        /Für Ben Brandt ist im September 2026 kein Soll hinterlegt\./,
      ),
    ).toHaveTextContent(
      "Die Krankmeldung zählt in diesem Monat deshalb 0 Stunden. Gilt später ein Soll für diese Tage, rechnet moto die Stunden nach. Das Soll legen Sie in der Personalakte unter „Arbeitszeitmodell“ fest.",
    );
    expect(mocks.args).toEqual(["7", "2026-09-09", true]);
  });

  it("nennt ohne Recht, wer das Soll einträgt", () => {
    mocks.hasTarget = false;
    renderNotice(false);

    expect(
      screen.getByText(
        /Für Ben Brandt ist im September 2026 kein Soll hinterlegt\./,
      ),
    ).toHaveTextContent("Das Soll trägt die Leitung ein.");
  });

  it("begrenzt die Aussage am Monatsende auf den geprüften Startmonat", () => {
    mocks.hasTarget = false;
    render(
      <MissingTargetDialogNotice
        staffId="7"
        staffName="Ben Brandt"
        date="2026-09-30"
        entryLabel="Die Abwesenheit"
        canSetTarget
      />,
    );

    expect(screen.getByText(/im September 2026 kein Soll/)).toHaveTextContent(
      "Die Abwesenheit zählt in diesem Monat deshalb 0 Stunden.",
    );
    expect(mocks.args).toEqual(["7", "2026-09-30", true]);
  });

  it.each([true, null])("bleibt still bei hasTarget %s", (value) => {
    mocks.hasTarget = value;
    const notice = () => (
      <MissingTargetDialogNotice
        staffId="7"
        staffName="Ben Brandt"
        date="2026-09-09"
        entryLabel="Die Krankmeldung"
        canSetTarget
      />
    );
    const { container, rerender } = render(notice());

    expect(container.firstChild).toBeNull();
    mocks.hasTarget = false;
    rerender(notice());
    expect(screen.getByText(/im September 2026 kein Soll/)).toBeVisible();
  });
});
