"use client";

/**
 * InstanceDetailSlideOver — right-side slide-over showing the full state
 * of a clicked instance plus lifecycle action buttons.
 *
 * Read-only MVP scope:
 * - All field rows are read-only display widgets (no editable inputs)
 * - Lifecycle actions (Starten / Beenden / Absagen) are wired to the
 *   real backend endpoints — they exist already (WP-B9)
 * - Edit (PUT /instances/{id}) and Spontan-Create (POST /instances) are
 *   shown as disabled buttons with German tooltips so the surface is
 *   visible but obviously inactive. Wiring these is the follow-up PR.
 */

import { useState } from "react";
import {
  CheckCircle2,
  CircleX,
  DoorOpen,
  Pencil,
  Play,
  Square,
  StickyNote,
  Timer,
  TriangleAlert,
  UserCheck,
  Users,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverClose,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import {
  getActivityTypeBadge,
  getGermanWeekdayLong,
  getStatusLabel,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance, InstanceStatus } from "~/lib/timetable-types";

export type LifecycleAction = "start" | "complete" | "cancel";

interface InstanceDetailSlideOverProps {
  instance: EnrichedInstance | null;
  onClose: () => void;
  onLifecycleAction: (action: LifecycleAction) => Promise<void>;
  /**
   * When true, edit + spontaneous-create UI surfaces are visible but
   * disabled with a tooltip. Default true until backend PUT/POST land.
   */
  editDeferred?: boolean;
}

function germanFullDate(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  const day = String(d.getDate()).padStart(2, "0");
  const month = String(d.getMonth() + 1).padStart(2, "0");
  return `${getGermanWeekdayLong(d)}, ${day}.${month}.${d.getFullYear()}`;
}

interface StatusBadgeProps {
  status: InstanceStatus;
}

function StatusBadge({ status }: StatusBadgeProps) {
  const palette: Record<InstanceStatus, { bg: string; text: string }> = {
    planned: { bg: "#F3F4F6", text: "#374151" },
    active: { bg: "#83CD2D", text: "#FFFFFF" },
    completed: { bg: "#E5E7EB", text: "#6B7280" },
    cancelled: { bg: "#FF3130", text: "#FFFFFF" },
  };
  const { bg, text } = palette[status];
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase"
      style={{ backgroundColor: bg, color: text }}
    >
      {status === "active" && (
        <span className="h-1.5 w-1.5 rounded-full bg-white" />
      )}
      {getStatusLabel(status)}
    </span>
  );
}

export function InstanceDetailSlideOver({
  instance,
  onClose,
  onLifecycleAction,
  editDeferred = true,
}: InstanceDetailSlideOverProps) {
  const [pendingAction, setPendingAction] = useState<LifecycleAction | null>(
    null,
  );

  const handleLifecycle = async (action: LifecycleAction) => {
    setPendingAction(action);
    try {
      await onLifecycleAction(action);
    } finally {
      setPendingAction(null);
    }
  };

  const open = instance !== null;

  return (
    <SlideOver
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      {instance && (
        <SlideOverContent>
          <SlideOverHeader>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <SlideOverTitle>{instance.title}</SlideOverTitle>
                  <StatusBadge status={instance.status} />
                  {(() => {
                    const tb = getActivityTypeBadge(instance.activityType);
                    return tb ? (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
                        style={{ backgroundColor: tb.bg }}
                      >
                        {tb.label}
                      </span>
                    ) : null;
                  })()}
                </div>
                <SlideOverDescription>
                  {germanFullDate(instance.date)} • {instance.startTime} –{" "}
                  {instance.endTime}
                </SlideOverDescription>
              </div>
              <SlideOverClose asChild>
                <button
                  type="button"
                  className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-200 text-slate-500 hover:bg-slate-100"
                  aria-label="Schließen"
                >
                  <X className="h-4 w-4" />
                </button>
              </SlideOverClose>
            </div>
          </SlideOverHeader>

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {instance.conflictWarnings.length > 0 && (
              <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] p-3">
                <div className="flex items-center gap-2 text-xs font-bold text-[#7F1D1D]">
                  <TriangleAlert className="h-4 w-4" />
                  {instance.conflictWarnings.length} Konflikt(e)
                </div>
                <ul className="mt-1 space-y-0.5 text-xs text-[#991B1B]">
                  {instance.conflictWarnings.map((w, i) => (
                    <li key={i}>• {w.message}</li>
                  ))}
                </ul>
              </div>
            )}

            <StatsRow instance={instance} />

            <Section title="Details">
              <Row icon={<Timer className="h-4 w-4" />} label="Zeit">
                {instance.startTime} – {instance.endTime}
              </Row>
              <Row icon={<DoorOpen className="h-4 w-4" />} label="Raum">
                {instance.roomName || `Raum #${instance.roomId}`}
              </Row>
              <Row
                icon={<UserCheck className="h-4 w-4" />}
                label={`Personal (${instance.staffCount})`}
              >
                {instance.staffCount === 0
                  ? "Niemand zugeordnet"
                  : `${instance.staffCount - instance.absentStaffCount} aktiv${
                      instance.absentStaffCount > 0
                        ? `, ${instance.absentStaffCount} abwesend`
                        : ""
                    }`}
              </Row>
              <Row icon={<Users className="h-4 w-4" />} label="Schüler">
                {instance.expectedStudentsCount + instance.presentStudentsCount}{" "}
                eingetragen
                {instance.presentStudentsCount > 0
                  ? ` • ${instance.presentStudentsCount} anwesend`
                  : ""}
              </Row>
              {instance.notes && (
                <Row icon={<StickyNote className="h-4 w-4" />} label="Notiz">
                  <span className="whitespace-pre-line">{instance.notes}</span>
                </Row>
              )}
            </Section>

            {editDeferred && (
              <div className="rounded-md border border-dashed border-slate-300 bg-slate-50 p-3 text-xs text-slate-500">
                Bearbeitung von Zeit, Raum und Personal kommt im nächsten
                Update. Aktuell: Lifecycle (Starten, Beenden, Absagen) ist live.
              </div>
            )}
          </div>

          <SlideOverFooter>
            <div className="flex flex-wrap items-center gap-2">
              {instance.status === "planned" && (
                <Button
                  variant="success"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("start")}
                  isLoading={pendingAction === "start"}
                  loadingText="Starte …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <Play className="h-4 w-4" />
                    Starten
                  </span>
                </Button>
              )}
              {instance.status === "active" && (
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("complete")}
                  isLoading={pendingAction === "complete"}
                  loadingText="Beende …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <Square className="h-4 w-4" />
                    Beenden
                  </span>
                </Button>
              )}
              {(instance.status === "planned" ||
                instance.status === "active") && (
                <Button
                  variant="outline_danger"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("cancel")}
                  isLoading={pendingAction === "cancel"}
                  loadingText="Sage ab …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <CircleX className="h-4 w-4" />
                    Absagen
                  </span>
                </Button>
              )}
              {instance.status === "completed" && (
                <span className="inline-flex items-center gap-2 text-xs text-slate-500">
                  <CheckCircle2 className="h-4 w-4" />
                  Diese Aktivität ist bereits abgeschlossen.
                </span>
              )}
              {instance.status === "cancelled" && (
                <span className="inline-flex items-center gap-2 text-xs text-slate-500">
                  <CircleX className="h-4 w-4" />
                  Diese Aktivität wurde abgesagt.
                </span>
              )}
            </div>
            {editDeferred && (
              <div className="flex items-center justify-end gap-2 text-xs text-slate-400">
                <Pencil className="h-3.5 w-3.5" />
                <span>Bearbeiten kommt im nächsten Update</span>
              </div>
            )}
          </SlideOverFooter>
        </SlideOverContent>
      )}
    </SlideOver>
  );
}

interface StatsRowProps {
  instance: EnrichedInstance;
}

function StatsRow({ instance }: StatsRowProps) {
  const expected = instance.expectedStudentsCount;
  const present = instance.presentStudentsCount;
  return (
    <div className="grid grid-cols-3 gap-2">
      <StatBox
        label="Anwesend"
        value={present > 0 ? `${present} / ${expected + present}` : "—"}
        color="#15803D"
        bg="#F0FDF4"
        border="#BBF7D0"
      />
      <StatBox
        label="Personal"
        value={`${instance.staffCount - instance.absentStaffCount} / ${instance.staffCount}`}
        color="#1E3A8A"
        bg="#EFF6FF"
        border="#BFDBFE"
      />
      <StatBox
        label="Status"
        value={getStatusLabel(instance.status)}
        color={instance.status === "cancelled" ? "#7F1D1D" : "#1F2937"}
        bg={instance.status === "cancelled" ? "#FEF2F2" : "#F8FAFC"}
        border={instance.status === "cancelled" ? "#FECACA" : "#E2E8F0"}
      />
    </div>
  );
}

interface StatBoxProps {
  label: string;
  value: string;
  color: string;
  bg: string;
  border: string;
}

function StatBox({ label, value, color, bg, border }: StatBoxProps) {
  return (
    <div
      className="rounded-md border px-3 py-2"
      style={{ backgroundColor: bg, borderColor: border }}
    >
      <div
        className="text-[10px] font-semibold tracking-wide uppercase"
        style={{ color }}
      >
        {label}
      </div>
      <div className="text-base font-bold" style={{ color }}>
        {value}
      </div>
    </div>
  );
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
}

function Section({ title, children }: SectionProps) {
  return (
    <div className="space-y-2">
      <h4 className="text-[10px] font-bold tracking-wider text-slate-400 uppercase">
        {title}
      </h4>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}

interface RowProps {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}

function Row({ icon, label, children }: RowProps) {
  return (
    <div className="flex items-start gap-3 text-sm">
      <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center text-slate-400">
        {icon}
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-[11px] font-medium text-slate-500">{label}</span>
        <span className="text-sm text-slate-900">{children}</span>
      </div>
    </div>
  );
}
