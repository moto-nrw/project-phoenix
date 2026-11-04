import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { DisplayClock } from "./display-clock";

afterEach(() => vi.unstubAllEnvs());
it.each(schoolDateCases("2026-05-27"))(
  "shows the Berlin school date in %s",
  (zone, instant) => {
    setSchoolDateClock(zone, instant, "2026-05-27");
    render(<DisplayClock />);
    expect(screen.getByText(/Mittwoch, 27. Mai/)).toBeInTheDocument();
  },
);
