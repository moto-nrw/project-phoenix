import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import ProtectedLoadingPage from "./loading";

describe("ProtectedLoadingPage", () => {
  it("renders the generic list-page skeleton", () => {
    render(<ProtectedLoadingPage />);

    expect(screen.getByLabelText("Laden...")).toBeInTheDocument();
  });
});
