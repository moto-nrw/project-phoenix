"use client";

import type { ReactNode } from "react";

import { LOCATION_COLORS } from "~/lib/location-helper";

export interface DataTableColumn<T> {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  headerClassName?: string;
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  emptyState?: ReactNode;
  headerActions?: ReactNode;
  caption?: string;
  isLoading?: boolean;
  rowClassName?: (row: T) => string;
}

const alignClass: Record<
  NonNullable<DataTableColumn<unknown>["align"]>,
  string
> = {
  left: "text-left",
  right: "text-right",
  center: "text-center",
};

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  emptyState,
  headerActions,
  caption,
  isLoading,
  rowClassName,
}: Readonly<DataTableProps<T>>) {
  const clickable = Boolean(onRowClick);

  return (
    <div className="w-full">
      {(caption ?? headerActions) && (
        <div className="mb-3 flex items-center justify-between gap-3">
          {caption ? (
            <p className="text-sm text-gray-600">{caption}</p>
          ) : (
            <span />
          )}
          {headerActions ? (
            <div className="flex items-center gap-2">{headerActions}</div>
          ) : null}
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-gray-200 bg-gray-50">
              {columns.map((col) => {
                const align = alignClass[col.align ?? "left"];
                return (
                  <th
                    key={col.key}
                    scope="col"
                    className={`px-4 py-3 text-xs font-semibold tracking-wider text-gray-500 uppercase ${align} ${col.headerClassName ?? ""}`}
                  >
                    {col.header}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-10 text-center text-sm text-gray-500"
                >
                  Wird geladen…
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-10 text-center text-sm text-gray-500"
                >
                  {emptyState ?? "Keine Einträge vorhanden."}
                </td>
              </tr>
            ) : (
              rows.map((row) => {
                const rowKey = getRowKey(row);
                const rowClasses = [
                  "border-b border-gray-100 last:border-b-0 transition-colors",
                  clickable ? "cursor-pointer hover:bg-gray-50" : "",
                  rowClassName ? rowClassName(row) : "",
                ]
                  .filter(Boolean)
                  .join(" ");
                return (
                  <tr
                    key={rowKey}
                    className={rowClasses}
                    onClick={onRowClick ? () => onRowClick(row) : undefined}
                    onKeyDown={
                      onRowClick
                        ? (event) => {
                            if (event.target !== event.currentTarget) {
                              return;
                            }
                            if (event.key === "Enter" || event.key === " ") {
                              event.preventDefault();
                              onRowClick(row);
                            }
                          }
                        : undefined
                    }
                    tabIndex={clickable ? 0 : undefined}
                    role={clickable ? "button" : undefined}
                  >
                    {columns.map((col) => {
                      const align = alignClass[col.align ?? "left"];
                      return (
                        <td
                          key={col.key}
                          className={`px-4 py-3 align-middle text-gray-900 ${align} ${col.className ?? ""}`}
                        >
                          {col.render(row)}
                        </td>
                      );
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

interface DataTableStatusBadgeProps {
  active: boolean;
  activeLabel?: string;
  inactiveLabel?: string;
}

export function DataTableStatusBadge({
  active,
  activeLabel = "Aktiv",
  inactiveLabel = "Inaktiv",
}: Readonly<DataTableStatusBadgeProps>) {
  const label = active ? activeLabel : inactiveLabel;
  const color = active ? LOCATION_COLORS.GROUP_ROOM : LOCATION_COLORS.HOME;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span style={{ color }}>{label}</span>
    </span>
  );
}
