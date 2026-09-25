# Runtime workload checkpoint-1-v1: baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.319 → 0.660 (-50.0%) | 1.826 → 0.889 (-51.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 2.874 → 1.789 (-37.7%) | 3.853 → 2.095 (-45.6%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.710 → 1.201 (-29.8%) | 2.117 → 1.546 (-27.0%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.list | 2.220 → 1.312 (-40.9%) | 2.584 → 1.473 (-43.0%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.070 → 1.341 (-35.2%) | 2.456 → 1.539 (-37.3%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.786 → 1.079 (-39.6%) | 2.095 → 1.317 (-37.1%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.556 → 1.583 (-38.1%) | 3.000 → 1.728 (-42.4%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.170 → 0.683 (-41.6%) | 1.415 → 0.897 (-36.6%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| school-calendar.periods | 1.880 → 1.263 (-32.8%) | 2.369 → 1.411 (-40.4%) | 180.000 → 210.000 (+16.7%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.520 → 0.859 (-43.5%) | 1.849 → 1.106 (-40.2%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.489 → 3.526 (+1.0%) | 3.943 → 3.870 (-1.8%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.732 → 1.668 (-38.9%) | 3.091 → 1.802 (-41.7%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 1.946 → 1.385 (-28.8%) | 2.574 → 1.950 (-24.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| meal-plan.list | 1.299 → 0.794 (-38.8%) | 1.474 → 1.000 (-32.2%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| feedback.list | 1.081 → 0.640 (-40.8%) | 1.219 → 0.984 (-19.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.877 → 2.704 (-30.3%) | 4.428 → 2.994 (-32.4%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.795 → 0.478 (-39.9%) | 0.981 → 0.708 (-27.8%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.796 → 0.483 (-39.3%) | 0.939 → 0.719 (-23.4%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.819 → 0.491 (-40.0%) | 0.948 → 0.620 (-34.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.017 → 0.019 (+10.5%) | 0.055 → 0.029 (-46.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.885 → 5.008 (-14.9%) | 6.503 → 5.674 (-12.8%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.380 → 4.231 (-21.3%) | 7.493 → 4.728 (-36.9%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.774 → 2.693 (-2.9%) | 3.336 → 2.965 (-11.1%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.488 → 3.520 (-21.6%) | 5.333 → 3.757 (-29.5%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 3.931 → 2.804 (-28.7%) | 4.597 → 3.200 (-30.4%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.500 → 0.871 (-42.0%) | 2.021 → 1.154 (-42.9%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 3.121 → 2.247 (-28.0%) | 4.711 → 2.409 (-48.9%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.864 → 1.270 (-31.9%) | 2.230 → 1.573 (-29.5%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.list | 2.220 → 1.555 (-30.0%) | 2.827 → 1.941 (-31.3%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.082 → 1.377 (-33.9%) | 2.476 → 1.606 (-35.1%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.868 → 1.173 (-37.2%) | 2.612 → 1.393 (-46.6%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.624 → 1.634 (-37.7%) | 3.095 → 1.820 (-41.2%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.203 → 0.695 (-42.2%) | 1.471 → 0.904 (-38.5%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-calendar.periods | 1.905 → 1.264 (-33.6%) | 2.378 → 1.489 (-37.4%) | 180.000 → 210.000 (+16.7%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.527 → 0.861 (-43.6%) | 1.945 → 1.120 (-42.4%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.526 → 3.660 (+3.8%) | 3.997 → 4.027 (+0.8%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.920 → 1.915 (-34.4%) | 4.593 → 2.115 (-53.9%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 2.197 → 1.725 (-21.5%) | 2.842 → 2.580 (-9.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 1.477 → 1.076 (-27.2%) | 1.780 → 1.239 (-30.4%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| feedback.list | 1.087 → 0.747 (-31.3%) | 1.316 → 0.995 (-24.4%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 4.213 → 2.725 (-35.3%) | 4.988 → 3.211 (-35.6%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.796 → 0.513 (-35.6%) | 0.991 → 0.769 (-22.4%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.808 → 0.675 (-16.5%) | 0.956 → 0.979 (+2.4%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.844 → 0.556 (-34.2%) | 1.020 → 0.722 (-29.2%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.018 → 0.024 (+33.1%) | 0.062 → 0.072 (+16.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 6.226 → 5.120 (-17.8%) | 7.591 → 6.156 (-18.9%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.736 → 4.620 (-19.5%) | 9.358 → 5.412 (-42.2%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.941 → 2.786 (-5.3%) | 4.499 → 3.005 (-33.2%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.621 → 3.536 (-23.5%) | 5.366 → 4.095 (-23.7%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.135 → 3.074 (-25.7%) | 4.691 → 3.440 (-26.7%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `timetable.not-found` `latency_p50_ms` median: 1.319 → 0.660
- `timetable.not-found` `latency_p50_ms` worst: 1.500 → 0.871
- `timetable.not-found` `latency_p95_ms` median: 1.826 → 0.889
- `timetable.not-found` `latency_p95_ms` worst: 2.021 → 1.154
- `organization-tenancy.resolve` `latency_p50_ms` median: 2.874 → 1.789
- `organization-tenancy.resolve` `latency_p50_ms` worst: 3.121 → 2.247
- `organization-tenancy.resolve` `latency_p95_ms` median: 3.853 → 2.095
- `organization-tenancy.resolve` `latency_p95_ms` worst: 4.711 → 2.409
- `facilities.update` `latency_p50_ms` median: 1.710 → 1.201
- `facilities.update` `latency_p50_ms` worst: 1.864 → 1.270
- `facilities.update` `latency_p95_ms` median: 2.117 → 1.546
- `facilities.update` `latency_p95_ms` worst: 2.230 → 1.573
- `facilities.list` `latency_p50_ms` median: 2.220 → 1.312
- `facilities.list` `latency_p50_ms` worst: 2.220 → 1.555
- `facilities.list` `latency_p95_ms` median: 2.584 → 1.473
- `facilities.list` `latency_p95_ms` worst: 2.827 → 1.941
- `school-structure.list` `latency_p50_ms` median: 2.070 → 1.341
- `school-structure.list` `latency_p50_ms` worst: 2.082 → 1.377
- `school-structure.list` `latency_p95_ms` median: 2.456 → 1.539
- `school-structure.list` `latency_p95_ms` worst: 2.476 → 1.606
- `people-directory.guardians` `latency_p50_ms` median: 1.786 → 1.079
- `people-directory.guardians` `latency_p50_ms` worst: 1.868 → 1.173
- `people-directory.guardians` `latency_p95_ms` median: 2.095 → 1.317
- `people-directory.guardians` `latency_p95_ms` worst: 2.612 → 1.393
- `timetable.activities` `latency_p50_ms` median: 2.556 → 1.583
- `timetable.activities` `latency_p50_ms` worst: 2.624 → 1.634
- `timetable.activities` `latency_p95_ms` median: 3.000 → 1.728
- `timetable.activities` `latency_p95_ms` worst: 3.095 → 1.820
- `timetable.categories` `latency_p50_ms` worst: 1.203 → 0.695
- `timetable.categories` `latency_p95_ms` median: 1.415 → 0.897
- `timetable.categories` `latency_p95_ms` worst: 1.471 → 0.904
- `school-calendar.periods` `latency_p50_ms` median: 1.880 → 1.263
- `school-calendar.periods` `latency_p50_ms` worst: 1.905 → 1.264
- `school-calendar.periods` `latency_p95_ms` median: 2.369 → 1.411
- `school-calendar.periods` `latency_p95_ms` worst: 2.378 → 1.489
- `school-calendar.periods` `queries_total` median: 180.000 → 210.000
- `school-calendar.periods` `queries_total` worst: 180.000 → 210.000
- `school-calendar.periods` `queries_min_per_operation` median: 6.000 → 7.000
- `school-calendar.periods` `queries_min_per_operation` worst: 6.000 → 7.000
- `school-calendar.periods` `queries_max_per_operation` median: 6.000 → 7.000
- `school-calendar.periods` `queries_max_per_operation` worst: 6.000 → 7.000
- `school-calendar.periods` `statements_with_row_counts` median: 120.000 → 150.000
- `school-calendar.periods` `statements_with_row_counts` worst: 120.000 → 150.000
- `school-calendar.periods` `module_rows_returned_or_changed` median: 30.000 → 60.000
- `school-calendar.periods` `module_rows_returned_or_changed` worst: 30.000 → 60.000
- `care-plan.offerings` `latency_p50_ms` median: 1.520 → 0.859
- `care-plan.offerings` `latency_p50_ms` worst: 1.527 → 0.861
- `care-plan.offerings` `latency_p95_ms` median: 1.849 → 1.106
- `care-plan.offerings` `latency_p95_ms` worst: 1.945 → 1.120
- `appointments.calendar` `latency_p50_ms` median: 2.732 → 1.668
- `appointments.calendar` `latency_p50_ms` worst: 2.920 → 1.915
- `appointments.calendar` `latency_p95_ms` median: 3.091 → 1.802
- `appointments.calendar` `latency_p95_ms` worst: 4.593 → 2.115
- `settings.schema` `latency_p50_ms` median: 1.946 → 1.385
- `settings.schema` `latency_p95_ms` median: 2.574 → 1.950
- `meal-plan.list` `latency_p50_ms` median: 1.299 → 0.794
- `meal-plan.list` `latency_p95_ms` worst: 1.780 → 1.239
- `school-membership.staff` `latency_p50_ms` median: 3.877 → 2.704
- `school-membership.staff` `latency_p50_ms` worst: 4.213 → 2.725
- `school-membership.staff` `latency_p95_ms` median: 4.428 → 2.994
- `school-membership.staff` `latency_p95_ms` worst: 4.988 → 3.211
- `school-membership.staff` `queries_total` median: 420.000 → 450.000
- `school-membership.staff` `queries_total` worst: 420.000 → 450.000
- `school-membership.staff` `queries_min_per_operation` median: 14.000 → 15.000
- `school-membership.staff` `queries_min_per_operation` worst: 14.000 → 15.000
- `school-membership.staff` `queries_max_per_operation` median: 14.000 → 15.000
- `school-membership.staff` `queries_max_per_operation` worst: 14.000 → 15.000
- `school-membership.staff` `driver_rows_returned_or_changed` median: 810.000 → 1170.000
- `school-membership.staff` `driver_rows_returned_or_changed` worst: 810.000 → 1170.000
- `school-membership.staff` `statements_with_row_counts` median: 360.000 → 390.000
- `school-membership.staff` `statements_with_row_counts` worst: 360.000 → 390.000
- `delivery.provider-unavailable` `queries_total` median: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_total` worst: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_min_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_min_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `statements_with_row_counts` median: 420.000 → 540.000
- `delivery.provider-unavailable` `statements_with_row_counts` worst: 420.000 → 540.000
- `delivery.provider-unavailable` `job_duration_total_ms` worst: 197.926 → 151.717
- `delivery.render-failure` `latency_p50_ms` median: 5.380 → 4.231
- `delivery.render-failure` `latency_p95_ms` median: 7.493 → 4.728
- `delivery.render-failure` `latency_p95_ms` worst: 9.358 → 5.412
- `delivery.render-failure` `queries_total` median: 720.000 → 960.000
- `delivery.render-failure` `queries_total` worst: 720.000 → 960.000
- `delivery.render-failure` `queries_min_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_min_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `statements_with_row_counts` median: 360.000 → 480.000
- `delivery.render-failure` `statements_with_row_counts` worst: 360.000 → 480.000
- `delivery.render-failure` `job_duration_total_ms` median: 167.643 → 128.061
- `delivery.render-failure` `job_duration_total_ms` worst: 178.263 → 140.273
- `delivery.idle` `latency_p95_ms` worst: 4.499 → 3.005
- `delivery.idle` `queries_total` median: 360.000 → 600.000
- `delivery.idle` `queries_total` worst: 360.000 → 600.000
- `delivery.idle` `queries_min_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_min_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `statements_with_row_counts` median: 180.000 → 300.000
- `delivery.idle` `statements_with_row_counts` worst: 180.000 → 300.000
- `timetable.materialize-create` `latency_p50_ms` median: 4.488 → 3.520
- `timetable.materialize-create` `latency_p50_ms` worst: 4.621 → 3.536
- `timetable.materialize-create` `latency_p95_ms` median: 5.333 → 3.757
- `timetable.materialize-create` `latency_p95_ms` worst: 5.366 → 4.095
- `timetable.materialize-create` `queries_total` median: 480.000 → 570.000
- `timetable.materialize-create` `queries_total` worst: 480.000 → 570.000
- `timetable.materialize-create` `queries_min_per_operation` median: 16.000 → 19.000
- `timetable.materialize-create` `queries_min_per_operation` worst: 16.000 → 19.000
- `timetable.materialize-create` `queries_max_per_operation` median: 16.000 → 19.000
- `timetable.materialize-create` `queries_max_per_operation` worst: 16.000 → 19.000
- `timetable.materialize-create` `driver_rows_returned_or_changed` median: 240.000 → 270.000
- `timetable.materialize-create` `driver_rows_returned_or_changed` worst: 240.000 → 270.000
- `timetable.materialize-create` `statements_with_row_counts` median: 420.000 → 510.000
- `timetable.materialize-create` `statements_with_row_counts` worst: 420.000 → 510.000
- `timetable.materialize-create` `job_duration_total_ms` median: 137.457 → 104.454
- `timetable.materialize-create` `job_duration_total_ms` worst: 141.620 → 106.384
- `timetable.materialize-existing` `latency_p50_ms` median: 3.931 → 2.804
- `timetable.materialize-existing` `latency_p50_ms` worst: 4.135 → 3.074
- `timetable.materialize-existing` `latency_p95_ms` median: 4.597 → 3.200
- `timetable.materialize-existing` `latency_p95_ms` worst: 4.691 → 3.440
- `timetable.materialize-existing` `queries_total` median: 450.000 → 510.000
- `timetable.materialize-existing` `queries_total` worst: 450.000 → 510.000
- `timetable.materialize-existing` `queries_min_per_operation` median: 15.000 → 17.000
- `timetable.materialize-existing` `queries_min_per_operation` worst: 15.000 → 17.000
- `timetable.materialize-existing` `queries_max_per_operation` median: 15.000 → 17.000
- `timetable.materialize-existing` `queries_max_per_operation` worst: 15.000 → 17.000
- `timetable.materialize-existing` `statements_with_row_counts` median: 390.000 → 450.000
- `timetable.materialize-existing` `statements_with_row_counts` worst: 390.000 → 450.000
- `timetable.materialize-existing` `module_rows_returned_or_changed` median: 150.000 → 120.000
- `timetable.materialize-existing` `module_rows_returned_or_changed` worst: 150.000 → 120.000
- `timetable.materialize-existing` `job_duration_total_ms` median: 118.932 → 85.297
- `timetable.materialize-existing` `job_duration_total_ms` worst: 124.835 → 92.791

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 784
- within-tolerance: 40
- material: 134
- coverage: 100

## Measured only in the candidate

These metrics have no baseline value and are not compared: `executed_write_rows_affected`.
