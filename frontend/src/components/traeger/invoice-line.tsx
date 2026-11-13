import type { ReactNode } from "react";

/**
 * One line of a monthly invoice: what it bills with a muted detail below,
 * and its amount at the right. Modules, reductions and Mensa months
 * (#4686) share it, so every line of an invoice reads the same.
 */
export function InvoiceLine({
  title,
  detail,
  amount,
  amountClassName = "text-gray-700",
}: Readonly<{
  title: ReactNode;
  detail: ReactNode;
  /** Left out when the invoice has a single line, whose sum stands beside it. */
  amount?: ReactNode;
  amountClassName?: string;
}>) {
  return (
    <li className="flex items-baseline justify-between gap-4 py-1.5">
      <div className="min-w-0">
        <p className="text-sm text-gray-900">{title}</p>
        <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-sm text-gray-600">
          {detail}
        </div>
      </div>
      {amount === undefined ? null : (
        <span className={`shrink-0 text-sm tabular-nums ${amountClassName}`}>
          {amount}
        </span>
      )}
    </li>
  );
}
