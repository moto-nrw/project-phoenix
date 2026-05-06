/**
 * Parent dashboard — placeholder. Commit 7 replaces this with the
 * cross-tenant children list. Kept here so commit 6's app shell has
 * a target route the auth guard can land on after login.
 */
export default function ParentDashboardPlaceholder() {
  return (
    <div className="space-y-3">
      <h1 className="text-2xl font-semibold text-gray-900">Eltern-Portal</h1>
      <p className="text-sm text-gray-600">
        Die Übersicht Ihrer Kinder wird in Kürze hier angezeigt.
      </p>
    </div>
  );
}
