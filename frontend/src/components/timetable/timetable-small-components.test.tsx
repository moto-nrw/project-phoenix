import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { InstanceBlock } from "./instance-block";
import { MonthPlannerGrid } from "./month-planner-grid";
import { PlanningMenu } from "./planning-menu";
import { TemplateCard } from "./template-card";
import { TemplateList } from "./template-list";
import { TimetableToolbar } from "./timetable-toolbar";
import { WeeklyCalendarGrid } from "./weekly-calendar-grid";
import { YearPlannerGrid } from "./year-planner-grid";
import type {
  EnrichedInstance,
  TimetableTemplate,
} from "~/lib/timetable-types";

const instance: EnrichedInstance = {
  id: "42",
  date: "2026-05-04",
  startTime: "12:00",
  endTime: "13:00",
  title: "Mensa",
  status: "active",
  isSpontaneous: false,
  isLive: true,
  activityType: "care",
  roomId: "3",
  roomName: "Mensa",
  staff: [],
  students: [],
  studentIds: ["21", "22"],
  staffCount: 1,
  absentStaffCount: 0,
  expectedStudentsCount: 2,
  presentStudentsCount: 1,
  conflictWarnings: [
    {
      kind: "room",
      resourceId: "3",
      message: "Raum doppelt belegt",
      canOverride: true,
    },
  ],
};

const template: TimetableTemplate = {
  id: "7",
  name: "Yoga",
  type: "activity",
  categoryId: "2",
  categoryName: "AG",
  roomId: "3",
  roomName: "Turnhalle",
  isOpen: true,
  maxParticipants: 12,
  enrollmentCount: 8,
  supervisorCount: 1,
  studentIds: ["21"],
  staffIds: ["11"],
  primaryStaffId: "11",
  schedules: [
    {
      id: "9",
      weekday: 1,
      startTime: "14:00",
      endTime: "15:00",
      weekPattern: 0,
      calendarPeriodId: "5",
    },
  ],
};

describe("small timetable components", () => {
  it("renders an instance block with live/conflict metadata and handles click", () => {
    const onClick = vi.fn();

    render(
      <div className="relative h-96">
        <InstanceBlock
          instance={instance}
          top={20}
          height={90}
          left="0%"
          width="100%"
          isSelected
          onClick={onClick}
        />
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: /mensa/i }));

    expect(onClick).toHaveBeenCalledOnce();
    expect(screen.getByLabelText("läuft")).toBeInTheDocument();
    expect(screen.getByLabelText("1 Konflikte")).toBeInTheDocument();
    expect(screen.getByText(/1 anwesend/)).toBeInTheDocument();
  });

  it("opens planning actions and confirms week replanning", async () => {
    const onMaterialize = vi.fn().mockResolvedValue(undefined);
    const onReplan = vi.fn().mockResolvedValue(undefined);

    render(
      <PlanningMenu
        onMaterialize={onMaterialize}
        onReplan={onReplan}
        weekLabel="KW 19"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /woche planen/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /lücken füllen/i }));
    await waitFor(() => expect(onMaterialize).toHaveBeenCalledOnce());

    fireEvent.click(screen.getByRole("button", { name: /woche planen/i }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /woche neu berechnen/i }),
    );
    expect(screen.getByRole("dialog")).toHaveTextContent("KW 19");

    fireEvent.click(screen.getByRole("button", { name: "Neu berechnen" }));

    await waitFor(() => expect(onReplan).toHaveBeenCalledOnce());
  });

  it("renders template cards/lists and wires create/edit/apply callbacks", () => {
    const onCreate = vi.fn();
    const onEdit = vi.fn();
    const onApply = vi.fn();

    const { rerender } = render(
      <TemplateList
        templates={[]}
        onCreate={onCreate}
        onEdit={onEdit}
        onApply={onApply}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /serientermin/i }));
    expect(onCreate).toHaveBeenCalledOnce();

    rerender(
      <TemplateList
        templates={[template]}
        onCreate={onCreate}
        onEdit={onEdit}
        onApply={onApply}
      />,
    );

    expect(screen.getByText("Yoga")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Termine erzeugen" }));

    expect(onEdit).toHaveBeenCalledWith(template);
    expect(onApply).toHaveBeenCalledWith(template);
  });

  it("renders a standalone template card with fallback labels", () => {
    const onEdit = vi.fn();
    const onApply = vi.fn();

    render(
      <TemplateCard
        template={{
          ...template,
          categoryName: "",
          roomName: undefined,
          enrollmentCount: 1,
          schedules: [],
        }}
        onEdit={onEdit}
        onApply={onApply}
      />,
    );

    expect(screen.getByText("Keine Zeiten hinterlegt")).toBeInTheDocument();
    expect(screen.getByText("Kein Raum")).toBeInTheDocument();
    expect(screen.getByText(/1 Kind/)).toBeInTheDocument();
  });

  it("handles toolbar view, navigation, add, and density controls", () => {
    const onViewChange = vi.fn();
    const onPrev = vi.fn();
    const onNext = vi.fn();
    const onToday = vi.fn();
    const onAddInstance = vi.fn();
    const onDensityChange = vi.fn();

    render(
      <TimetableToolbar
        view="week"
        onViewChange={onViewChange}
        rangeLabel="KW 19"
        onPrev={onPrev}
        onNext={onNext}
        onToday={onToday}
        density="normal"
        onDensityChange={onDensityChange}
        onAddInstance={onAddInstance}
        planWeekAction={<span>Plan action</span>}
      />,
    );

    fireEvent.click(screen.getByRole("tab", { name: "Monat" }));
    fireEvent.click(screen.getByLabelText("Vorheriger Zeitraum"));
    fireEvent.click(screen.getByLabelText("Nächster Zeitraum"));
    fireEvent.click(screen.getByRole("button", { name: "Heute" }));
    fireEvent.click(screen.getByRole("button", { name: "Termin" }));
    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    fireEvent.click(screen.getByRole("menuitemradio", { name: "Komfortabel" }));

    expect(onViewChange).toHaveBeenCalledWith("month");
    expect(onPrev).toHaveBeenCalledOnce();
    expect(onNext).toHaveBeenCalledOnce();
    expect(onToday).toHaveBeenCalledOnce();
    expect(onAddInstance).toHaveBeenCalledOnce();
    expect(onDensityChange).toHaveBeenCalledWith("comfortable");
  });

  it("renders week/month/year grids and forwards date selections", () => {
    const onInstanceClick = vi.fn();
    const onDayClick = vi.fn();
    const onMonthClick = vi.fn();
    const weekDays = [
      new Date("2026-05-04T00:00:00"),
      new Date("2026-05-05T00:00:00"),
      new Date("2026-05-06T00:00:00"),
      new Date("2026-05-07T00:00:00"),
      new Date("2026-05-08T00:00:00"),
    ];

    const { rerender } = render(
      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={[instance]}
        selectedId="42"
        onInstanceClick={onInstanceClick}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={60}
        emptyState={{ title: "Keine Termine", description: "Noch nichts da." }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /mensa/i }));
    expect(onInstanceClick).toHaveBeenCalledWith(instance);
    expect(screen.getByText("Keine Termine")).toBeInTheDocument();

    rerender(
      <MonthPlannerGrid
        days={weekDays}
        monthDate={new Date("2026-05-01T00:00:00")}
        instances={[instance]}
        todayISO="2026-05-04"
        onDayClick={onDayClick}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /4/i }));
    expect(onDayClick).toHaveBeenCalledWith("2026-05-04");

    rerender(
      <YearPlannerGrid
        months={[new Date("2026-05-01T00:00:00")]}
        instances={[instance]}
        todayISO="2026-05-04"
        onMonthClick={onMonthClick}
        onDayClick={onDayClick}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /mai/i }));
    fireEvent.click(screen.getByRole("button", { name: /2026-05-04/ }));

    expect(onMonthClick).toHaveBeenCalledWith(new Date("2026-05-01T00:00:00"));
    expect(onDayClick).toHaveBeenCalledWith("2026-05-04");
  });
});
