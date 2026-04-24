import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const receivedProps = vi.fn();

vi.mock("./pickup-schedule-manager", () => ({
  default: (props: Record<string, unknown>) => {
    receivedProps(props);
    return <div data-testid="pickup-schedule-manager" />;
  },
}));

import { StudentAbholungTab } from "./student-abholung-tab";

describe("StudentAbholungTab", () => {
  it("forwards all props to PickupScheduleManager", () => {
    const onUpdate = vi.fn();
    render(
      <StudentAbholungTab
        studentId="42"
        isSick={true}
        isExcused={false}
        onUpdate={onUpdate}
      />,
    );

    expect(screen.getByTestId("pickup-schedule-manager")).toBeInTheDocument();
    expect(receivedProps).toHaveBeenCalledWith({
      studentId: "42",
      isSick: true,
      isExcused: false,
      onUpdate,
    });
  });

  it("forwards undefined optional props as-is", () => {
    render(<StudentAbholungTab studentId="5" />);
    expect(receivedProps).toHaveBeenCalledWith(
      expect.objectContaining({
        studentId: "5",
        isSick: undefined,
        isExcused: undefined,
        onUpdate: undefined,
      }),
    );
  });
});
