# Runtime workload checkpoint-1-v1: baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.177 → 0.660 (-43.9%) | 1.374 → 0.889 (-35.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| organization-tenancy.resolve | 2.763 → 1.789 (-35.2%) | 3.186 → 2.095 (-34.2%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.637 → 1.201 (-26.7%) | 1.934 → 1.546 (-20.0%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 2.074 → 1.312 (-36.7%) | 2.448 → 1.473 (-39.8%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 1.991 → 1.341 (-32.6%) | 2.319 → 1.539 (-33.6%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.680 → 1.079 (-35.8%) | 1.878 → 1.317 (-29.9%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.511 → 1.583 (-36.9%) | 2.902 → 1.728 (-40.5%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.104 → 0.683 (-38.1%) | 1.277 → 0.897 (-29.8%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.990 → 1.263 (-36.5%) | 2.374 → 1.411 (-40.5%) | 240.000 → 210.000 (-12.5%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.453 → 0.859 (-40.9%) | 1.629 → 1.106 (-32.1%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.026 → 3.526 (+16.5%) | 3.394 → 3.870 (+14.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.627 → 1.668 (-36.5%) | 3.194 → 1.802 (-43.6%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 1.990 → 1.385 (-30.4%) | 2.446 → 1.950 (-20.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| meal-plan.list | 1.288 → 0.794 (-38.3%) | 1.520 → 1.000 (-34.2%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| feedback.list | 1.098 → 0.640 (-41.7%) | 1.224 → 0.984 (-19.6%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.619 → 2.704 (-25.3%) | 4.188 → 2.994 (-28.5%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.850 → 0.478 (-43.7%) | 0.977 → 0.708 (-27.5%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.813 → 0.483 (-40.6%) | 0.984 → 0.719 (-26.9%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.852 → 0.491 (-42.3%) | 1.013 → 0.620 (-38.8%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.021 → 0.019 (-10.7%) | 0.035 → 0.029 (-16.1%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.687 → 5.008 (-11.9%) | 6.324 → 5.674 (-10.3%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.231 → 4.231 (-19.1%) | 6.103 → 4.728 (-22.5%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.847 → 2.693 (-5.4%) | 3.305 → 2.965 (-10.3%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.779 → 3.520 (-26.3%) | 5.411 → 3.757 (-30.6%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.168 → 2.804 (-32.7%) | 4.728 → 3.200 (-32.3%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.549 → 0.871 (-43.8%) | 2.783 → 1.154 (-58.5%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 2.837 → 2.247 (-20.8%) | 3.219 → 2.409 (-25.2%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.673 → 1.270 (-24.1%) | 1.967 → 1.573 (-20.1%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 2.125 → 1.555 (-26.8%) | 2.516 → 1.941 (-22.9%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.059 → 1.377 (-33.1%) | 2.351 → 1.606 (-31.7%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.681 → 1.173 (-30.2%) | 1.890 → 1.393 (-26.3%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| timetable.activities | 2.513 → 1.634 (-35.0%) | 3.313 → 1.820 (-45.1%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.151 → 0.695 (-39.6%) | 1.324 → 0.904 (-31.7%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 2.078 → 1.264 (-39.2%) | 2.690 → 1.489 (-44.6%) | 240.000 → 210.000 (-12.5%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.460 → 0.861 (-41.0%) | 1.826 → 1.120 (-38.7%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.168 → 3.660 (+15.5%) | 3.785 → 4.027 (+6.4%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.833 → 1.915 (-32.4%) | 3.499 → 2.115 (-39.5%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 2.041 → 1.725 (-15.5%) | 2.583 → 2.580 (-0.1%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 1.513 → 1.076 (-28.9%) | 2.186 → 1.239 (-43.3%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| feedback.list | 1.179 → 0.747 (-36.6%) | 1.408 → 0.995 (-29.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.871 → 2.725 (-29.6%) | 4.319 → 3.211 (-25.6%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.876 → 0.513 (-41.5%) | 1.083 → 0.769 (-29.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.875 → 0.675 (-22.9%) | 1.049 → 0.979 (-6.7%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.906 → 0.556 (-38.6%) | 1.217 → 0.722 (-40.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.022 → 0.024 (+8.8%) | 0.096 → 0.072 (-24.9%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 6.252 → 5.120 (-18.1%) | 6.736 → 6.156 (-8.6%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.695 → 4.620 (-18.9%) | 6.473 → 5.412 (-16.4%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 3.013 → 2.786 (-7.5%) | 3.541 → 3.005 (-15.1%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.930 → 3.536 (-28.3%) | 6.298 → 4.095 (-35.0%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.356 → 3.074 (-29.4%) | 5.039 → 3.440 (-31.7%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `timetable.not-found` `latency_p50_ms` median: 1.177 → 0.660
- `timetable.not-found` `latency_p50_ms` worst: 1.549 → 0.871
- `timetable.not-found` `latency_p95_ms` worst: 2.783 → 1.154
- `organization-tenancy.resolve` `latency_p50_ms` median: 2.763 → 1.789
- `organization-tenancy.resolve` `latency_p50_ms` worst: 2.837 → 2.247
- `organization-tenancy.resolve` `latency_p95_ms` median: 3.186 → 2.095
- `organization-tenancy.resolve` `latency_p95_ms` worst: 3.219 → 2.409
- `facilities.list` `latency_p50_ms` median: 2.074 → 1.312
- `facilities.list` `latency_p50_ms` worst: 2.125 → 1.555
- `facilities.list` `latency_p95_ms` median: 2.448 → 1.473
- `facilities.list` `latency_p95_ms` worst: 2.516 → 1.941
- `school-structure.list` `latency_p50_ms` median: 1.991 → 1.341
- `school-structure.list` `latency_p50_ms` worst: 2.059 → 1.377
- `school-structure.list` `latency_p95_ms` median: 2.319 → 1.539
- `school-structure.list` `latency_p95_ms` worst: 2.351 → 1.606
- `people-directory.guardians` `latency_p50_ms` median: 1.680 → 1.079
- `people-directory.guardians` `latency_p50_ms` worst: 1.681 → 1.173
- `people-directory.guardians` `latency_p95_ms` median: 1.878 → 1.317
- `timetable.activities` `latency_p50_ms` median: 2.511 → 1.583
- `timetable.activities` `latency_p50_ms` worst: 2.513 → 1.634
- `timetable.activities` `latency_p95_ms` median: 2.902 → 1.728
- `timetable.activities` `latency_p95_ms` worst: 3.313 → 1.820
- `school-calendar.periods` `latency_p50_ms` median: 1.990 → 1.263
- `school-calendar.periods` `latency_p50_ms` worst: 2.078 → 1.264
- `school-calendar.periods` `latency_p95_ms` median: 2.374 → 1.411
- `school-calendar.periods` `latency_p95_ms` worst: 2.690 → 1.489
- `school-calendar.periods` `queries_total` median: 240.000 → 210.000
- `school-calendar.periods` `queries_total` worst: 240.000 → 210.000
- `school-calendar.periods` `queries_min_per_operation` median: 8.000 → 7.000
- `school-calendar.periods` `queries_min_per_operation` worst: 8.000 → 7.000
- `school-calendar.periods` `queries_max_per_operation` median: 8.000 → 7.000
- `school-calendar.periods` `queries_max_per_operation` worst: 8.000 → 7.000
- `school-calendar.periods` `statements_with_row_counts` median: 180.000 → 150.000
- `school-calendar.periods` `statements_with_row_counts` worst: 180.000 → 150.000
- `school-calendar.periods` `module_rows_returned_or_changed` median: 30.000 → 60.000
- `school-calendar.periods` `module_rows_returned_or_changed` worst: 30.000 → 60.000
- `care-plan.offerings` `latency_p50_ms` median: 1.453 → 0.859
- `care-plan.offerings` `latency_p50_ms` worst: 1.460 → 0.861
- `care-plan.offerings` `latency_p95_ms` median: 1.629 → 1.106
- `care-plan.offerings` `latency_p95_ms` worst: 1.826 → 1.120
- `appointments.calendar` `latency_p50_ms` median: 2.627 → 1.668
- `appointments.calendar` `latency_p50_ms` worst: 2.833 → 1.915
- `appointments.calendar` `latency_p95_ms` median: 3.194 → 1.802
- `appointments.calendar` `latency_p95_ms` worst: 3.499 → 2.115
- `settings.schema` `latency_p50_ms` median: 1.990 → 1.385
- `meal-plan.list` `latency_p95_ms` median: 1.520 → 1.000
- `meal-plan.list` `latency_p95_ms` worst: 2.186 → 1.239
- `school-membership.staff` `latency_p50_ms` median: 3.619 → 2.704
- `school-membership.staff` `latency_p50_ms` worst: 3.871 → 2.725
- `school-membership.staff` `latency_p95_ms` median: 4.188 → 2.994
- `school-membership.staff` `latency_p95_ms` worst: 4.319 → 3.211
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
- `delivery.render-failure` `latency_p95_ms` median: 6.103 → 4.728
- `delivery.render-failure` `queries_total` median: 720.000 → 960.000
- `delivery.render-failure` `queries_total` worst: 720.000 → 960.000
- `delivery.render-failure` `queries_min_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_min_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `statements_with_row_counts` median: 360.000 → 480.000
- `delivery.render-failure` `statements_with_row_counts` worst: 360.000 → 480.000
- `delivery.render-failure` `job_duration_total_ms` median: 161.401 → 128.061
- `delivery.idle` `queries_total` median: 360.000 → 600.000
- `delivery.idle` `queries_total` worst: 360.000 → 600.000
- `delivery.idle` `queries_min_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_min_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `statements_with_row_counts` median: 180.000 → 300.000
- `delivery.idle` `statements_with_row_counts` worst: 180.000 → 300.000
- `timetable.materialize-create` `latency_p50_ms` median: 4.779 → 3.520
- `timetable.materialize-create` `latency_p50_ms` worst: 4.930 → 3.536
- `timetable.materialize-create` `latency_p95_ms` median: 5.411 → 3.757
- `timetable.materialize-create` `latency_p95_ms` worst: 6.298 → 4.095
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
- `timetable.materialize-create` `job_duration_total_ms` median: 148.741 → 104.454
- `timetable.materialize-create` `job_duration_total_ms` worst: 151.395 → 106.384
- `timetable.materialize-existing` `latency_p50_ms` median: 4.168 → 2.804
- `timetable.materialize-existing` `latency_p50_ms` worst: 4.356 → 3.074
- `timetable.materialize-existing` `latency_p95_ms` median: 4.728 → 3.200
- `timetable.materialize-existing` `latency_p95_ms` worst: 5.039 → 3.440
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
- `timetable.materialize-existing` `job_duration_total_ms` median: 126.350 → 85.297
- `timetable.materialize-existing` `job_duration_total_ms` worst: 130.946 → 92.791

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 824
- within-tolerance: 55
- material: 119
- coverage: 100
