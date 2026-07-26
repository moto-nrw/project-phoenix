import type { ReactNode } from "react";

export interface CustomValueField {
  type: string;
  target?: string;
  options?: Array<{ label: string; value: string }>;
}

const WEEKDAYS = [
  ["mon", "Mo"],
  ["tue", "Di"],
  ["wed", "Mi"],
  ["thu", "Do"],
  ["fri", "Fr"],
] as const;

function formatStringValue(
  v: string,
  field?: CustomValueField,
): ReactNode | null {
  const trimmed = v.trim();
  if (trimmed === "") return null;
  if (field?.type === "select" && field.options) {
    const opt = field.options.find((o) => o.value === trimmed);
    if (opt) return opt.label;
  }
  return trimmed;
}

function formatWeekdayObject(
  o: Record<string, unknown>,
  field?: CustomValueField,
): ReactNode | null {
  const isWeekdayMultiMode =
    field?.type === "weekday_multi_mode" ||
    WEEKDAYS.some(([key]) => Array.isArray(o[key]));
  if (isWeekdayMultiMode) {
    const modeLabels: Record<string, string> = {
      alone: "geht zu Fuß",
      bus: "fährt Bus",
      pickup: "wird abgeholt",
      accompanied: "geht mit anderem Kind",
    };
    const parts = WEEKDAYS.flatMap(([key, label]) => {
      const raw = o[key];
      if (!Array.isArray(raw)) return [];
      const modes = raw
        .filter((mode): mode is string => typeof mode === "string")
        .map((mode) => modeLabels[mode] ?? mode);
      return modes.length > 0 ? [`${label}: ${modes.join(", ")}`] : [];
    });
    return parts.length > 0 ? parts.join("; ") : null;
  }

  const isWeekdayMode =
    field?.type === "weekday_mode" ||
    WEEKDAYS.some(
      ([key]) =>
        o[key] === "bus" ||
        o[key] === "pickup" ||
        o[key] === "alone" ||
        o[key] === "accompanied",
    );
  if (isWeekdayMode) {
    const modeLabels: Record<string, string> = {
      alone: "geht zu Fuß",
      bus: "fährt Bus",
      pickup: "wird abgeholt",
      accompanied: "geht mit anderem Kind",
    };
    // "alone" is the implicit default rendered by the "Geht immer alleine"
    // fallback below, so only non-alone days are listed explicitly. accompanied
    // (#1694) MUST be listed here — dropping it would mislabel a child who
    // never goes alone as "Geht immer alleine".
    const parts = WEEKDAYS.filter(
      ([key]) =>
        o[key] === "bus" || o[key] === "pickup" || o[key] === "accompanied",
    ).map(([key, label]) => `${label}: ${modeLabels[o[key] as string]}`);
    if (parts.length > 0) return parts.join(", ");
    if (field?.type === "weekday_mode") return "Geht immer alleine";
    return null;
  }

  const isWeekdayBoolean =
    field?.type === "weekday_boolean" ||
    WEEKDAYS.some(([key]) => typeof o[key] === "boolean");
  if (isWeekdayBoolean) {
    const days = WEEKDAYS.filter(([key]) => o[key] === true).map(
      ([, label]) => label,
    );
    if (days.length > 0) return days.join(", ");
    if (field?.target === "student.pickup_status") {
      return "Geht alleine nach Hause";
    }
    return null;
  }

  const cells = WEEKDAYS.map(([key, label]) => ({
    label,
    value: o[key],
  })).filter(
    (c) => typeof c.value === "string" && (c.value as string).trim() !== "",
  );
  if (cells.length === 0) return null;
  return (
    <span>
      {cells.map((c) => (
        <span key={c.label} className="mr-3 inline-block">
          <span className="text-gray-500">{c.label}:</span>{" "}
          <span className="font-medium">{c.value as string}</span>
        </span>
      ))}
    </span>
  );
}

export function formatCustomValue(
  v: unknown,
  field?: CustomValueField,
): ReactNode | null {
  if (v === null || v === undefined) return null;
  if (typeof v === "boolean") return v ? "Ja" : "Nein";
  if (typeof v === "string") return formatStringValue(v, field);
  if (typeof v === "number") return String(v);

  if (Array.isArray(v)) {
    if (v.length === 0) return null;
    return (
      <ul className="space-y-1 text-sm">
        {v.map((row) => (
          <li key={JSON.stringify(row)} className="text-gray-800">
            {formatStructuredEntry(row)}
          </li>
        ))}
      </ul>
    );
  }
  if (typeof v === "object") {
    return formatWeekdayObject(v as Record<string, unknown>, field);
  }
  return String(v);
}

function formatStructuredEntry(row: unknown): ReactNode {
  if (!row || typeof row !== "object") return String(row);
  const r = row as Record<string, unknown>;

  if (typeof r.first_name === "string" || typeof r.last_name === "string") {
    const parts: string[] = [];
    const fullName = `${typeof r.first_name === "string" ? r.first_name : ""} ${
      typeof r.last_name === "string" ? r.last_name : ""
    }`.trim();
    if (fullName) parts.push(fullName);
    if (typeof r.relationship_type === "string" && r.relationship_type) {
      parts[parts.length - 1] += ` (${r.relationship_type})`;
    }
    if (typeof r.email === "string" && r.email) parts.push(r.email);
    if (Array.isArray(r.phone_numbers)) {
      const phones = (r.phone_numbers as Array<Record<string, unknown>>)
        .map((p) => (typeof p.phone_number === "string" ? p.phone_number : ""))
        .filter(Boolean);
      if (phones.length > 0) parts.push(phones.join(", "));
    }
    if (r.can_pickup === true) parts.push("Abholberechtigt");
    if (r.is_emergency_contact === true) parts.push("Notfallkontakt");
    return parts.join(", ");
  }

  if (typeof r.phone_number === "string") {
    const labelMap: Record<string, string> = {
      mobile: "Mobil",
      home: "Privat",
      work: "Arbeit",
      other: "Sonstige",
    };
    const t =
      typeof r.phone_type === "string" && r.phone_type in labelMap
        ? labelMap[r.phone_type as string]
        : null;
    return `${r.phone_number}${t ? ` (${t})` : ""}${
      r.is_primary === true ? " (Hauptnummer)" : ""
    }`;
  }

  return JSON.stringify(r);
}
