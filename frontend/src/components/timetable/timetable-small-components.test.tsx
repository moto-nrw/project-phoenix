import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { InstanceBlock } from "./instance-block";
import { MonthPlannerGrid } from "./month-planner-grid";
import { TemplateCard } from "./template-card";
import { TemplateList } from "./template-list";
import { TimetableAddMenu } from "./timetable-add-menu";
import { TimetableRatioPill } from "./timetable-ratio-pill";
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
  requiredStaffCount: 1,
  assignedStaffCount: 1,
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
  targetGroupType: "none",
  enrollmentCount: 8,
  supervisorCount: 1,
  requiredStaffCount: 1,
  assignedStaffCount: 1,
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

  it("marks spontaneous instances in week and month grids", () => {
    const spontaneous = {
      ...instance,
      status: "planned" as const,
      isLive: false,
      isSpontaneous: true,
      conflictWarnings: [],
    };
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
        instances={[spontaneous]}
        selectedId={null}
        onInstanceClick={vi.fn()}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={90}
      />,
    );

    expect(screen.getByText("Spontan")).toBeInTheDocument();

    rerender(
      <MonthPlannerGrid
        days={weekDays}
        monthDate={new Date("2026-05-01T00:00:00")}
        instances={[spontaneous]}
        todayISO="2026-05-04"
        onDayClick={vi.fn()}
      />,
    );

    expect(screen.getByText("Spontan")).toBeInTheDocument();
  });

  it("offers only one-off and recurring entries in the add menu", () => {
    const onAddInstance = vi.fn();
    const onAddSeries = vi.fn();

    render(
      <TimetableAddMenu
        onAddInstance={onAddInstance}
        onAddSeries={onAddSeries}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /neu/i }));
    expect(screen.getAllByRole("menuitem")).toHaveLength(2);
    expect(
      screen.queryByText("Fehlende Termine eintragen"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Regeltermine neu aufbauen"),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("menuitem", { name: /einmaliger termin/i }),
    );
    expect(onAddInstance).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole("button", { name: /neu/i }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /regelmäßiger termin/i }),
    );
    expect(onAddSeries).toHaveBeenCalledOnce();
  });

  it("renders template cards/lists and wires create/edit/apply callbacks", () => {
    const onCreate = vi.fn();
    const onEdit = vi.fn();
    const onApply = vi.fn();
    const onArchive = vi.fn();

    const { rerender } = render(
      <TemplateList
        templates={[]}
        onCreate={onCreate}
        onEdit={onEdit}
        onApply={onApply}
        onArchive={onArchive}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /regeltermin/i }));
    expect(onCreate).toHaveBeenCalledOnce();

    rerender(
      <TemplateList
        templates={[template]}
        onCreate={onCreate}
        onEdit={onEdit}
        onApply={onApply}
        onArchive={onArchive}
      />,
    );

    expect(screen.getByText("Yoga")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Termine erzeugen" }));

    expect(onEdit).toHaveBeenCalledWith(template);
    expect(onArchive).toHaveBeenCalledWith(template);
    expect(onApply).toHaveBeenCalledWith(template);
  });

  it("renders a standalone template card with fallback labels", () => {
    const onEdit = vi.fn();
    const onApply = vi.fn();
    const onArchive = vi.fn();

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
        onArchive={onArchive}
      />,
    );

    expect(screen.getByText("Keine Zeiten hinterlegt")).toBeInTheDocument();
    expect(screen.getByText("Kein Raum")).toBeInTheDocument();
    expect(screen.getByText(/1 Kind/)).toBeInTheDocument();
  });

  it("shows staffing capacity on template cards", () => {
    render(
      <TemplateCard
        template={{
          ...template,
          requiredStaffCount: 3,
          assignedStaffCount: 1,
        }}
        onEdit={vi.fn()}
        onApply={vi.fn()}
        onArchive={vi.fn()}
      />,
    );

    expect(screen.getByText("1/3")).toBeInTheDocument();
    expect(screen.getByText("Besetzung:")).toHaveClass("sr-only");
  });

  it("gives compact ratio pills a screen-reader label", () => {
    render(
      <TimetableRatioPill
        icon={<span />}
        label="Besetzung"
        value="1/3"
        tone="warning"
        variant="compact"
      />,
    );

    expect(screen.getByText("Besetzung:")).toHaveClass("sr-only");
    expect(screen.getByText("1/3")).toBeVisible();
  });

  it("shows a missing-room hint only when the template has no room", () => {
    const { rerender } = render(
      <TemplateCard
        template={template}
        onEdit={vi.fn()}
        onApply={vi.fn()}
        onArchive={vi.fn()}
      />,
    );

    expect(
      screen.queryByText("Raum fehlt – wird nicht eingeplant"),
    ).not.toBeInTheDocument();

    rerender(
      <TemplateCard
        template={{ ...template, roomId: undefined, roomName: undefined }}
        onEdit={vi.fn()}
        onApply={vi.fn()}
        onArchive={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Raum fehlt – wird nicht eingeplant"),
    ).toBeInTheDocument();
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

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Monat" }), {
      button: 0,
    });
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

  it("closes the toolbar density menu from keyboard and outside clicks", () => {
    render(
      <TimetableToolbar
        view="week"
        onViewChange={vi.fn()}
        rangeLabel="KW 19"
        onPrev={vi.fn()}
        onNext={vi.fn()}
        onToday={vi.fn()}
        density="normal"
        onDensityChange={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("offers period management in the toolbar overflow menu in every view", () => {
    const onManagePeriods = vi.fn();
    const sharedProps = {
      onViewChange: vi.fn(),
      rangeLabel: "Mai 2026",
      onPrev: vi.fn(),
      onNext: vi.fn(),
      onToday: vi.fn(),
      onManagePeriods,
    };

    const { rerender } = render(
      <TimetableToolbar view="month" {...sharedProps} />,
    );

    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    // Density only applies to the week view; outside it the menu carries
    // just the Verwaltung section.
    expect(screen.queryByText("Zeilenhöhe")).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("menuitem", { name: /schuljahre & ferien/i }),
    );
    expect(onManagePeriods).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();

    rerender(<TimetableToolbar view="series" {...sharedProps} navDisabled />);
    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    expect(
      screen.getByRole("menuitem", { name: /schuljahre & ferien/i }),
    ).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });

    rerender(
      <TimetableToolbar
        view="week"
        {...sharedProps}
        density="normal"
        onDensityChange={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByLabelText("Weitere Optionen"));
    expect(screen.getByText("Zeilenhöhe")).toBeInTheDocument();
    expect(screen.getByText("Verwaltung")).toBeInTheDocument();
  });

  it("hides irrelevant toolbar controls for series and today states", () => {
    render(
      <TimetableToolbar
        view="series"
        onViewChange={vi.fn()}
        rangeLabel="Regeltermine"
        onPrev={vi.fn()}
        onNext={vi.fn()}
        onToday={vi.fn()}
        isOnToday
        navDisabled
      />,
    );

    expect(
      screen.queryByLabelText("Vorheriger Zeitraum"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Heute" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Weitere Optionen")).not.toBeInTheDocument();
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

  it("includes annual staffing and conflict status in day labels", () => {
    render(
      <YearPlannerGrid
        months={[new Date("2026-05-01T00:00:00")]}
        instances={[
          {
            ...instance,
            requiredStaffCount: 3,
            assignedStaffCount: 1,
          },
          {
            ...instance,
            id: "cancelled",
            status: "cancelled",
            requiredStaffCount: 2,
            assignedStaffCount: 0,
            conflictWarnings: [],
          },
        ]}
        onMonthClick={vi.fn()}
        onDayClick={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", {
        name: "2026-05-04: 2 Termine, 1 unterbesetzter Termin, 1 Konflikt",
      }),
    ).toBeInTheDocument();
  });

  it("keeps week events outside configured hours reachable", () => {
    const weekDays = [
      new Date("2026-05-04T00:00:00"),
      new Date("2026-05-05T00:00:00"),
      new Date("2026-05-06T00:00:00"),
      new Date("2026-05-07T00:00:00"),
      new Date("2026-05-08T00:00:00"),
    ];
    const early: EnrichedInstance = {
      ...instance,
      id: "early",
      title: "Frühtermin",
      startTime: "08:00",
      endTime: "09:00",
    };
    const late: EnrichedInstance = {
      ...instance,
      id: "late",
      title: "Spättermin",
      startTime: "18:30",
      endTime: "19:00",
    };

    render(
      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={[early, late]}
        selectedId={null}
        onInstanceClick={vi.fn()}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={60}
      />,
    );

    expect(screen.getByRole("button", { name: /frühtermin/i })).toHaveStyle({
      top: "0px",
    });
    expect(screen.getByRole("button", { name: /spättermin/i })).toHaveStyle({
      top: "630px",
    });
  });

  it("renders hourly slot click targets when onSlotClick is provided", () => {
    const onSlotClick = vi.fn();
    const onInstanceClick = vi.fn();
    const weekDays = [
      new Date("2026-05-04T00:00:00"),
      new Date("2026-05-05T00:00:00"),
      new Date("2026-05-06T00:00:00"),
      new Date("2026-05-07T00:00:00"),
      new Date("2026-05-08T00:00:00"),
    ];

    render(
      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={[instance]}
        selectedId={null}
        onInstanceClick={onInstanceClick}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={60}
        onSlotClick={onSlotClick}
      />,
    );

    // 8 full hours (09:00–16:00) per day, 5 visible days
    const slotButtons = screen.getAllByRole("button", {
      name: /Neuen Termin anlegen/,
    });
    expect(slotButtons).toHaveLength(40);
    expect(slotButtons[0]).toHaveAttribute(
      "aria-label",
      "Neuen Termin anlegen: Mo 04.05., 09:00 Uhr",
    );
    // Slots stay out of the tab order — they would add ~40 stops before
    // the first event; keyboard users go through the "Neu" menu.
    expect(slotButtons[0]).toHaveAttribute("tabindex", "-1");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Neuen Termin anlegen: Di 05.05., 14:00 Uhr",
      }),
    );
    expect(onSlotClick).toHaveBeenCalledWith("2026-05-05", 14);

    // Instance blocks keep their own click handlers with slots present.
    fireEvent.click(screen.getByRole("button", { name: /mensa/i }));
    expect(onInstanceClick).toHaveBeenCalledWith(instance);
    expect(onSlotClick).toHaveBeenCalledTimes(1);
  });

  it("renders no slot click targets without onSlotClick", () => {
    render(
      <WeeklyCalendarGrid
        weekDays={[new Date("2026-05-04T00:00:00")]}
        instances={[instance]}
        selectedId={null}
        onInstanceClick={vi.fn()}
        todayISO="2026-05-04"
        dayStartHour={9}
        dayEndHour={17}
        hourHeightPx={60}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /Neuen Termin anlegen/ }),
    ).not.toBeInTheDocument();
  });
});
