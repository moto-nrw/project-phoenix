# Runtime workload checkpoint-1-v1: baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.319 → 0.668 (-49.4%) | 1.826 → 0.933 (-48.9%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 2.874 → 1.730 (-39.8%) | 3.853 → 1.913 (-50.3%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.710 → 1.006 (-41.2%) | 2.117 → 1.235 (-41.7%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.list | 2.220 → 1.187 (-46.5%) | 2.584 → 1.452 (-43.8%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.070 → 1.326 (-36.0%) | 2.456 → 1.557 (-36.6%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.786 → 1.093 (-38.8%) | 2.095 → 1.318 (-37.1%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.556 → 1.641 (-35.8%) | 3.000 → 1.896 (-36.8%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.170 → 0.681 (-41.8%) | 1.415 → 0.889 (-37.2%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| school-calendar.periods | 1.880 → 1.292 (-31.3%) | 2.369 → 1.506 (-36.4%) | 180.000 → 210.000 (+16.7%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.520 → 0.849 (-44.1%) | 1.849 → 1.098 (-40.6%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.489 → 3.589 (+2.8%) | 3.943 → 4.014 (+1.8%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.732 → 1.778 (-34.9%) | 3.091 → 1.954 (-36.8%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 1.946 → 1.421 (-26.9%) | 2.574 → 1.857 (-27.8%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| meal-plan.list | 1.299 → 0.806 (-38.0%) | 1.474 → 1.052 (-28.6%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 1.081 → 0.640 (-40.8%) | 1.219 → 0.886 (-27.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.877 → 2.656 (-31.5%) | 4.428 → 2.961 (-33.1%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.795 → 0.480 (-39.7%) | 0.981 → 0.721 (-26.5%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.796 → 0.489 (-38.5%) | 0.939 → 0.695 (-26.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.819 → 0.481 (-41.2%) | 0.948 → 0.709 (-25.2%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.017 → 0.020 (+18.3%) | 0.055 → 0.057 (+2.2%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.885 → 4.711 (-19.9%) | 6.503 → 5.684 (-12.6%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.380 → 4.383 (-18.5%) | 7.493 → 4.738 (-36.8%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.774 → 2.784 (+0.3%) | 3.336 → 3.079 (-7.7%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.488 → 3.392 (-24.4%) | 5.333 → 3.779 (-29.1%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 3.931 → 2.865 (-27.1%) | 4.597 → 3.098 (-32.6%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.500 → 0.920 (-38.7%) | 2.021 → 1.445 (-28.5%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 3.121 → 2.304 (-26.2%) | 4.711 → 2.899 (-38.5%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.864 → 1.152 (-38.2%) | 2.230 → 1.575 (-29.4%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.list | 2.220 → 1.340 (-39.6%) | 2.827 → 1.562 (-44.7%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.082 → 1.359 (-34.7%) | 2.476 → 1.629 (-34.2%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.868 → 1.182 (-36.7%) | 2.612 → 1.787 (-31.6%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.624 → 1.897 (-27.7%) | 3.095 → 2.095 (-32.3%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.203 → 0.761 (-36.7%) | 1.471 → 1.038 (-29.4%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.905 → 1.349 (-29.1%) | 2.378 → 1.648 (-30.7%) | 180.000 → 210.000 (+16.7%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.527 → 0.900 (-41.0%) | 1.945 → 1.178 (-39.4%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.526 → 3.824 (+8.4%) | 3.997 → 4.929 (+23.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| appointments.calendar | 2.920 → 2.029 (-30.5%) | 4.593 → 2.365 (-48.5%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 2.197 → 1.586 (-27.8%) | 2.842 → 2.097 (-26.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| meal-plan.list | 1.477 → 1.021 (-30.8%) | 1.780 → 1.174 (-34.1%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| feedback.list | 1.087 → 0.827 (-24.0%) | 1.316 → 1.034 (-21.4%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 4.213 → 2.668 (-36.7%) | 4.988 → 3.376 (-32.3%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.796 → 0.482 (-39.4%) | 0.991 → 0.751 (-24.2%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.808 → 0.506 (-37.4%) | 0.956 → 0.832 (-13.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.844 → 0.498 (-41.0%) | 1.020 → 0.796 (-22.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.018 → 0.020 (+11.5%) | 0.062 → 0.058 (-6.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 6.226 → 4.774 (-23.3%) | 7.591 → 6.950 (-8.5%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.736 → 4.533 (-21.0%) | 9.358 → 5.650 (-39.6%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.941 → 2.878 (-2.1%) | 4.499 → 3.097 (-31.2%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.621 → 3.621 (-21.6%) | 5.366 → 4.623 (-13.8%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.135 → 2.871 (-30.6%) | 4.691 → 3.335 (-28.9%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `timetable.not-found` `latency_p50_ms` median: 1.319 → 0.668
- `timetable.not-found` `latency_p50_ms` worst: 1.500 → 0.920
- `timetable.not-found` `latency_p95_ms` median: 1.826 → 0.933
- `timetable.not-found` `latency_p95_ms` worst: 2.021 → 1.445
- `organization-tenancy.resolve` `latency_p50_ms` median: 2.874 → 1.730
- `organization-tenancy.resolve` `latency_p50_ms` worst: 3.121 → 2.304
- `organization-tenancy.resolve` `latency_p95_ms` median: 3.853 → 1.913
- `organization-tenancy.resolve` `latency_p95_ms` worst: 4.711 → 2.899
- `facilities.update` `latency_p50_ms` median: 1.710 → 1.006
- `facilities.update` `latency_p50_ms` worst: 1.864 → 1.152
- `facilities.update` `latency_p95_ms` median: 2.117 → 1.235
- `facilities.update` `latency_p95_ms` worst: 2.230 → 1.575
- `facilities.list` `latency_p50_ms` median: 2.220 → 1.187
- `facilities.list` `latency_p50_ms` worst: 2.220 → 1.340
- `facilities.list` `latency_p95_ms` median: 2.584 → 1.452
- `facilities.list` `latency_p95_ms` worst: 2.827 → 1.562
- `school-structure.list` `latency_p50_ms` median: 2.070 → 1.326
- `school-structure.list` `latency_p50_ms` worst: 2.082 → 1.359
- `school-structure.list` `latency_p95_ms` median: 2.456 → 1.557
- `school-structure.list` `latency_p95_ms` worst: 2.476 → 1.629
- `people-directory.guardians` `latency_p50_ms` median: 1.786 → 1.093
- `people-directory.guardians` `latency_p50_ms` worst: 1.868 → 1.182
- `people-directory.guardians` `latency_p95_ms` median: 2.095 → 1.318
- `people-directory.guardians` `latency_p95_ms` worst: 2.612 → 1.787
- `timetable.activities` `latency_p50_ms` median: 2.556 → 1.641
- `timetable.activities` `latency_p50_ms` worst: 2.624 → 1.897
- `timetable.activities` `latency_p95_ms` median: 3.000 → 1.896
- `timetable.activities` `latency_p95_ms` worst: 3.095 → 2.095
- `timetable.categories` `latency_p95_ms` median: 1.415 → 0.889
- `school-calendar.periods` `latency_p50_ms` median: 1.880 → 1.292
- `school-calendar.periods` `latency_p50_ms` worst: 1.905 → 1.349
- `school-calendar.periods` `latency_p95_ms` median: 2.369 → 1.506
- `school-calendar.periods` `latency_p95_ms` worst: 2.378 → 1.648
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
- `care-plan.offerings` `latency_p50_ms` median: 1.520 → 0.849
- `care-plan.offerings` `latency_p50_ms` worst: 1.527 → 0.900
- `care-plan.offerings` `latency_p95_ms` median: 1.849 → 1.098
- `care-plan.offerings` `latency_p95_ms` worst: 1.945 → 1.178
- `communication.messages` `latency_p95_ms` worst: 3.997 → 4.929
- `appointments.calendar` `latency_p50_ms` median: 2.732 → 1.778
- `appointments.calendar` `latency_p50_ms` worst: 2.920 → 2.029
- `appointments.calendar` `latency_p95_ms` median: 3.091 → 1.954
- `appointments.calendar` `latency_p95_ms` worst: 4.593 → 2.365
- `settings.schema` `latency_p50_ms` median: 1.946 → 1.421
- `settings.schema` `latency_p50_ms` worst: 2.197 → 1.586
- `settings.schema` `latency_p95_ms` median: 2.574 → 1.857
- `settings.schema` `latency_p95_ms` worst: 2.842 → 2.097
- `meal-plan.list` `latency_p95_ms` worst: 1.780 → 1.174
- `school-membership.staff` `latency_p50_ms` median: 3.877 → 2.656
- `school-membership.staff` `latency_p50_ms` worst: 4.213 → 2.668
- `school-membership.staff` `latency_p95_ms` median: 4.428 → 2.961
- `school-membership.staff` `latency_p95_ms` worst: 4.988 → 3.376
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
- `delivery.provider-unavailable` `latency_p50_ms` worst: 6.226 → 4.774
- `delivery.provider-unavailable` `queries_total` median: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_total` worst: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_min_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_min_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `statements_with_row_counts` median: 420.000 → 540.000
- `delivery.provider-unavailable` `statements_with_row_counts` worst: 420.000 → 540.000
- `delivery.provider-unavailable` `job_duration_total_ms` median: 178.451 → 141.285
- `delivery.provider-unavailable` `job_duration_total_ms` worst: 197.926 → 150.553
- `delivery.render-failure` `latency_p50_ms` worst: 5.736 → 4.533
- `delivery.render-failure` `latency_p95_ms` median: 7.493 → 4.738
- `delivery.render-failure` `latency_p95_ms` worst: 9.358 → 5.650
- `delivery.render-failure` `queries_total` median: 720.000 → 960.000
- `delivery.render-failure` `queries_total` worst: 720.000 → 960.000
- `delivery.render-failure` `queries_min_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_min_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `statements_with_row_counts` median: 360.000 → 480.000
- `delivery.render-failure` `statements_with_row_counts` worst: 360.000 → 480.000
- `delivery.render-failure` `job_duration_total_ms` median: 167.643 → 132.047
- `delivery.render-failure` `job_duration_total_ms` worst: 178.263 → 137.810
- `delivery.idle` `latency_p95_ms` worst: 4.499 → 3.097
- `delivery.idle` `queries_total` median: 360.000 → 600.000
- `delivery.idle` `queries_total` worst: 360.000 → 600.000
- `delivery.idle` `queries_min_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_min_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `statements_with_row_counts` median: 180.000 → 300.000
- `delivery.idle` `statements_with_row_counts` worst: 180.000 → 300.000
- `timetable.materialize-create` `latency_p50_ms` median: 4.488 → 3.392
- `timetable.materialize-create` `latency_p50_ms` worst: 4.621 → 3.621
- `timetable.materialize-create` `latency_p95_ms` median: 5.333 → 3.779
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
- `timetable.materialize-create` `job_duration_total_ms` median: 137.457 → 103.154
- `timetable.materialize-create` `job_duration_total_ms` worst: 141.620 → 111.769
- `timetable.materialize-existing` `latency_p50_ms` median: 3.931 → 2.865
- `timetable.materialize-existing` `latency_p50_ms` worst: 4.135 → 2.871
- `timetable.materialize-existing` `latency_p95_ms` median: 4.597 → 3.098
- `timetable.materialize-existing` `latency_p95_ms` worst: 4.691 → 3.335
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
- `timetable.materialize-existing` `job_duration_total_ms` median: 118.932 → 86.354
- `timetable.materialize-existing` `job_duration_total_ms` worst: 124.835 → 88.005

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 784
- within-tolerance: 39
- material: 135
- coverage: 100

## Measured only in the candidate

These metrics have no baseline value and are not compared: `executed_write_rows_affected`.
