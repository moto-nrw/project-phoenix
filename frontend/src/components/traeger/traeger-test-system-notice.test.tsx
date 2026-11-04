import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TraegerTestSystemNotice } from "./traeger-test-system-notice";

describe("TraegerTestSystemNotice", () => {
  it("marks a test Träger as a test system with invented children", () => {
    render(<TraegerTestSystemNotice />);

    expect(screen.getByText("Testsystem")).toBeInTheDocument();
    expect(screen.getByText(/Alle Kinder sind erfunden/)).toBeInTheDocument();
  });
});
