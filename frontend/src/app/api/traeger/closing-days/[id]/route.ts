import { withPrivateNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut, traeger } from "~/lib/traeger/route-wrapper.server";

// Ein Schließtag des Trägers (#4702): ändern oder löschen.
const closingDayPath = (params: Record<string, unknown>) =>
  `/traeger/closing-days/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(closingDayPath);

// Das Löschen bestätigt mit dem Zeichen der Vorschau, dass die Eltern der
// betroffenen Ferienbuchungen von den wieder geöffneten Tagen erfahren, und
// antwortet mit der Zahl dieser Familien (#6457).
export const DELETE = withPrivateNoStore(
  traeger.route(async (request, _token, params) => {
    const token = request.nextUrl.searchParams.get("holiday_bookings_token");
    const query = token
      ? `?${new URLSearchParams({ holiday_bookings_token: token }).toString()}`
      : "";
    return traeger.forwardToBackend(
      request,
      `${closingDayPath(params)}${query}`,
      {
        method: "DELETE",
      },
    );
  }),
);
