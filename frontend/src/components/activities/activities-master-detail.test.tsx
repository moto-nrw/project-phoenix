import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    sections,
    onSubmit,
    onCancel,
  }: {
    sections: Array<{
      fields: Array<{ name: string; disabled?: boolean; helperText?: string }>;
    }>;
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
  }) => {
    const nameField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "name");

    return (
      <div data-testid="database-form">
        {nameField?.disabled ? (
          <span data-testid="name-disabled">{nameField.helperText}</span>
        ) : null}
        <button
          type="button"
          onClick={() => void onSubmit({ name: "Updated Activity" })}
        >
          Save
        </button>
        <button type="button" onClick={onCancel}>
          Cancel
        </button>
      </div>
    );
  },
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { ActivitiesMasterDetail } from "./activities-master-detail";
import type { Activity } from "@/lib/activity-helpers";

const regularActivity: Activity = {
  id: "1",
  name: "Mathe AG",
  max_participant: 12,
  is_open_ags: false,
  supervisor_id: "1",
  supervisor_name: "Herr Fischer",
  ag_category_id: "1",
  category_name: "Sport",
  created_at: new Date(),
  updated_at: new Date(),
};

const systemActivity: Activity = {
  ...regularActivity,
  id: "2",
  name: "Schulhof Freispiel",
  supervisor_name: undefined,
};

describe("ActivitiesMasterDetail", () => {
  const onSelect = vi.fn();
  const onSaveActivity = vi.fn();
  const onResetForm = vi.fn();
  const onDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity]}
        selectedId={null}
        selectedActivity={null}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="none:0"
      />,
    );

    expect(screen.queryByTestId("database-form")).not.toBeInTheDocument();
    expect(screen.getByText("Mathe AG")).toBeInTheDocument();
  });

  it("renders the selected activity detail form and save path", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity]}
        selectedId="1"
        selectedActivity={regularActivity}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="1:0"
      />,
    );

    expect(screen.getAllByText("Mathe AG")).toHaveLength(2);
    expect(screen.getByTestId("database-form")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Save"));
    expect(onSaveActivity).toHaveBeenCalledWith({ name: "Updated Activity" });
  });

  it("hides the delete action for system activities and disables the name field", () => {
    render(
      <ActivitiesMasterDetail
        activities={[systemActivity]}
        selectedId="2"
        selectedActivity={systemActivity}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="2:0"
      />,
    );

    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
    expect(screen.getByTestId("name-disabled")).toHaveTextContent(
      "Systemaktivität: Name kann nicht geändert werden",
    );
  });

  it("passes delete clicks through for regular activities", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity]}
        selectedId="1"
        selectedActivity={regularActivity}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="1:0"
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();
  });

  it("renders the supervisor name when present", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity]}
        selectedId="1"
        selectedActivity={regularActivity}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="1:0"
      />,
    );

    expect(screen.getByText("Hauptbetreuer")).toBeInTheDocument();
    expect(screen.getByText("Herr Fischer")).toBeInTheDocument();
  });

  it("groups activities under a single 'Alle Aktivitäten' bucket and renders a Stammdaten tab", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity, systemActivity]}
        selectedId="1"
        selectedActivity={regularActivity}
        detailLoading={false}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="1:0"
      />,
    );

    expect(screen.getByText("Alle Aktivitäten")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("renders the loading spinner instead of the form while detailLoading is true", () => {
    render(
      <ActivitiesMasterDetail
        activities={[regularActivity]}
        selectedId="1"
        selectedActivity={regularActivity}
        detailLoading={true}
        onSelect={onSelect}
        onSaveActivity={onSaveActivity}
        onResetForm={onResetForm}
        onDeleteClick={onDeleteClick}
        formResetKey="1:0"
      />,
    );

    expect(
      screen.getByText("Aktivitätsdaten werden geladen..."),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("database-form")).not.toBeInTheDocument();
  });
});
