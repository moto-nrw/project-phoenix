# Runtime workload checkpoint-1-v1: baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.177 → 0.668 (-43.2%) | 1.374 → 0.933 (-32.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| organization-tenancy.resolve | 2.763 → 1.730 (-37.4%) | 3.186 → 1.913 (-39.9%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.update | 1.637 → 1.006 (-38.6%) | 1.934 → 1.235 (-36.1%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| facilities.list | 2.074 → 1.187 (-42.8%) | 2.448 → 1.452 (-40.7%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 1.991 → 1.326 (-33.4%) | 2.319 → 1.557 (-32.8%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.680 → 1.093 (-35.0%) | 1.878 → 1.318 (-29.8%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.activities | 2.511 → 1.641 (-34.7%) | 2.902 → 1.896 (-34.7%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.104 → 0.681 (-38.3%) | 1.277 → 0.889 (-30.4%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.990 → 1.292 (-35.1%) | 2.374 → 1.506 (-36.6%) | 240.000 → 210.000 (-12.5%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.453 → 0.849 (-41.5%) | 1.629 → 1.098 (-32.6%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.026 → 3.589 (+18.6%) | 3.394 → 4.014 (+18.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 2.627 → 1.778 (-32.3%) | 3.194 → 1.954 (-38.8%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 1.990 → 1.421 (-28.6%) | 2.446 → 1.857 (-24.1%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| meal-plan.list | 1.288 → 0.806 (-37.4%) | 1.520 → 1.052 (-30.8%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 1.098 → 0.640 (-41.7%) | 1.224 → 0.886 (-27.6%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.619 → 2.656 (-26.6%) | 4.188 → 2.961 (-29.3%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.850 → 0.480 (-43.6%) | 0.977 → 0.721 (-26.2%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.813 → 0.489 (-39.8%) | 0.984 → 0.695 (-29.3%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.852 → 0.481 (-43.5%) | 1.013 → 0.709 (-30.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.021 → 0.020 (-4.3%) | 0.035 → 0.057 (+60.9%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.687 → 4.711 (-17.2%) | 6.324 → 5.684 (-10.1%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.231 → 4.383 (-16.2%) | 6.103 → 4.738 (-22.4%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 2.847 → 2.784 (-2.2%) | 3.305 → 3.079 (-6.8%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.779 → 3.392 (-29.0%) | 5.411 → 3.779 (-30.2%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.168 → 2.865 (-31.3%) | 4.728 → 3.098 (-34.5%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 1.549 → 0.920 (-40.6%) | 2.783 → 1.445 (-48.1%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| organization-tenancy.resolve | 2.837 → 2.304 (-18.8%) | 3.219 → 2.899 (-10.0%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.update | 1.673 → 1.152 (-31.1%) | 1.967 → 1.575 (-20.0%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| facilities.list | 2.125 → 1.340 (-36.9%) | 2.516 → 1.562 (-37.9%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| school-structure.list | 2.059 → 1.359 (-34.0%) | 2.351 → 1.629 (-30.7%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| people-directory.guardians | 1.681 → 1.182 (-29.7%) | 1.890 → 1.787 (-5.5%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.activities | 2.513 → 1.897 (-24.5%) | 3.313 → 2.095 (-36.7%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| timetable.categories | 1.151 → 0.761 (-33.9%) | 1.324 → 1.038 (-21.6%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 2.078 → 1.349 (-35.1%) | 2.690 → 1.648 (-38.7%) | 240.000 → 210.000 (-12.5%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| care-plan.offerings | 1.460 → 0.900 (-38.3%) | 1.826 → 1.178 (-35.5%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| communication.messages | 3.168 → 3.824 (+20.7%) | 3.785 → 4.929 (+30.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| appointments.calendar | 2.833 → 2.029 (-28.4%) | 3.499 → 2.365 (-32.4%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| settings.schema | 2.041 → 1.586 (-22.3%) | 2.583 → 2.097 (-18.8%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 1.513 → 1.021 (-32.5%) | 2.186 → 1.174 (-46.3%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| feedback.list | 1.179 → 0.827 (-29.9%) | 1.408 → 1.034 (-26.6%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 3.871 → 2.668 (-31.1%) | 4.319 → 3.376 (-21.8%) | 420.000 → 450.000 (+7.1%) | 810.000 → 1170.000 (+44.4%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| facilities.invalid-id | 0.876 → 0.482 (-45.0%) | 1.083 → 0.751 (-30.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.875 → 0.506 (-42.2%) | 1.049 → 0.832 (-20.7%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.906 → 0.498 (-45.0%) | 1.217 → 0.796 (-34.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.022 → 0.020 (-8.8%) | 0.096 → 0.058 (-39.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 6.252 → 4.774 (-23.6%) | 6.736 → 6.950 (+3.2%) | 780.000 → 1020.000 (+30.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.render-failure | 5.695 → 4.533 (-20.4%) | 6.473 → 5.650 (-12.7%) | 720.000 → 960.000 (+33.3%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| delivery.idle | 3.013 → 2.878 (-4.5%) | 3.541 → 3.097 (-12.5%) | 360.000 → 600.000 (+66.7%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-create | 4.930 → 3.621 (-26.6%) | 6.298 → 4.623 (-26.6%) | 480.000 → 570.000 (+18.8%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | driver_rows_returned_or_changed, job_duration_total_ms, latency_p50_ms, latency_p95_ms, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |
| timetable.materialize-existing | 4.356 → 2.871 (-34.1%) | 5.039 → 3.335 (-33.8%) | 450.000 → 510.000 (+13.3%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms, module_rows_returned_or_changed, queries_max_per_operation, queries_min_per_operation, queries_total, statements_with_row_counts |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `timetable.not-found` `latency_p50_ms` median: 1.177 → 0.668
- `timetable.not-found` `latency_p50_ms` worst: 1.549 → 0.920
- `timetable.not-found` `latency_p95_ms` worst: 2.783 → 1.445
- `organization-tenancy.resolve` `latency_p50_ms` median: 2.763 → 1.730
- `organization-tenancy.resolve` `latency_p95_ms` median: 3.186 → 1.913
- `facilities.update` `latency_p50_ms` median: 1.637 → 1.006
- `facilities.update` `latency_p50_ms` worst: 1.673 → 1.152
- `facilities.update` `latency_p95_ms` median: 1.934 → 1.235
- `facilities.list` `latency_p50_ms` median: 2.074 → 1.187
- `facilities.list` `latency_p50_ms` worst: 2.125 → 1.340
- `facilities.list` `latency_p95_ms` median: 2.448 → 1.452
- `facilities.list` `latency_p95_ms` worst: 2.516 → 1.562
- `school-structure.list` `latency_p50_ms` median: 1.991 → 1.326
- `school-structure.list` `latency_p50_ms` worst: 2.059 → 1.359
- `school-structure.list` `latency_p95_ms` median: 2.319 → 1.557
- `school-structure.list` `latency_p95_ms` worst: 2.351 → 1.629
- `people-directory.guardians` `latency_p50_ms` median: 1.680 → 1.093
- `people-directory.guardians` `latency_p95_ms` median: 1.878 → 1.318
- `timetable.activities` `latency_p50_ms` median: 2.511 → 1.641
- `timetable.activities` `latency_p50_ms` worst: 2.513 → 1.897
- `timetable.activities` `latency_p95_ms` median: 2.902 → 1.896
- `timetable.activities` `latency_p95_ms` worst: 3.313 → 2.095
- `school-calendar.periods` `latency_p50_ms` median: 1.990 → 1.292
- `school-calendar.periods` `latency_p50_ms` worst: 2.078 → 1.349
- `school-calendar.periods` `latency_p95_ms` median: 2.374 → 1.506
- `school-calendar.periods` `latency_p95_ms` worst: 2.690 → 1.648
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
- `care-plan.offerings` `latency_p50_ms` median: 1.453 → 0.849
- `care-plan.offerings` `latency_p50_ms` worst: 1.460 → 0.900
- `care-plan.offerings` `latency_p95_ms` median: 1.629 → 1.098
- `care-plan.offerings` `latency_p95_ms` worst: 1.826 → 1.178
- `communication.messages` `latency_p50_ms` worst: 3.168 → 3.824
- `communication.messages` `latency_p95_ms` worst: 3.785 → 4.929
- `appointments.calendar` `latency_p50_ms` median: 2.627 → 1.778
- `appointments.calendar` `latency_p50_ms` worst: 2.833 → 2.029
- `appointments.calendar` `latency_p95_ms` median: 3.194 → 1.954
- `appointments.calendar` `latency_p95_ms` worst: 3.499 → 2.365
- `settings.schema` `latency_p50_ms` median: 1.990 → 1.421
- `settings.schema` `latency_p95_ms` median: 2.446 → 1.857
- `meal-plan.list` `latency_p95_ms` worst: 2.186 → 1.174
- `school-membership.staff` `latency_p50_ms` median: 3.619 → 2.656
- `school-membership.staff` `latency_p50_ms` worst: 3.871 → 2.668
- `school-membership.staff` `latency_p95_ms` median: 4.188 → 2.961
- `school-membership.staff` `latency_p95_ms` worst: 4.319 → 3.376
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
- `delivery.provider-unavailable` `latency_p50_ms` worst: 6.252 → 4.774
- `delivery.provider-unavailable` `queries_total` median: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_total` worst: 780.000 → 1020.000
- `delivery.provider-unavailable` `queries_min_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_min_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` median: 26.000 → 34.000
- `delivery.provider-unavailable` `queries_max_per_operation` worst: 26.000 → 34.000
- `delivery.provider-unavailable` `statements_with_row_counts` median: 420.000 → 540.000
- `delivery.provider-unavailable` `statements_with_row_counts` worst: 420.000 → 540.000
- `delivery.render-failure` `latency_p50_ms` worst: 5.695 → 4.533
- `delivery.render-failure` `latency_p95_ms` median: 6.103 → 4.738
- `delivery.render-failure` `queries_total` median: 720.000 → 960.000
- `delivery.render-failure` `queries_total` worst: 720.000 → 960.000
- `delivery.render-failure` `queries_min_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_min_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` median: 24.000 → 32.000
- `delivery.render-failure` `queries_max_per_operation` worst: 24.000 → 32.000
- `delivery.render-failure` `statements_with_row_counts` median: 360.000 → 480.000
- `delivery.render-failure` `statements_with_row_counts` worst: 360.000 → 480.000
- `delivery.render-failure` `job_duration_total_ms` worst: 174.270 → 137.810
- `delivery.idle` `queries_total` median: 360.000 → 600.000
- `delivery.idle` `queries_total` worst: 360.000 → 600.000
- `delivery.idle` `queries_min_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_min_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` median: 12.000 → 20.000
- `delivery.idle` `queries_max_per_operation` worst: 12.000 → 20.000
- `delivery.idle` `statements_with_row_counts` median: 180.000 → 300.000
- `delivery.idle` `statements_with_row_counts` worst: 180.000 → 300.000
- `timetable.materialize-create` `latency_p50_ms` median: 4.779 → 3.392
- `timetable.materialize-create` `latency_p50_ms` worst: 4.930 → 3.621
- `timetable.materialize-create` `latency_p95_ms` median: 5.411 → 3.779
- `timetable.materialize-create` `latency_p95_ms` worst: 6.298 → 4.623
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
- `timetable.materialize-create` `job_duration_total_ms` median: 148.741 → 103.154
- `timetable.materialize-create` `job_duration_total_ms` worst: 151.395 → 111.769
- `timetable.materialize-existing` `latency_p50_ms` median: 4.168 → 2.865
- `timetable.materialize-existing` `latency_p50_ms` worst: 4.356 → 2.871
- `timetable.materialize-existing` `latency_p95_ms` median: 4.728 → 3.098
- `timetable.materialize-existing` `latency_p95_ms` worst: 5.039 → 3.335
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
- `timetable.materialize-existing` `job_duration_total_ms` median: 126.350 → 86.354
- `timetable.materialize-existing` `job_duration_total_ms` worst: 130.946 → 88.005

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 824
- within-tolerance: 51
- material: 123
- coverage: 100
