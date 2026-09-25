# Runtime workload checkpoint-1-v1 → checkpoint-1-v2 (bridge): baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 0.668 → 0.821 (+22.9%) | 0.933 → 1.054 (+13.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| organization-tenancy.resolve | 1.730 → 1.855 (+7.2%) | 1.913 → 2.333 (+22.0%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.update | 1.006 → 1.092 (+8.6%) | 1.235 → 1.379 (+11.7%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 1.187 → 1.335 (+12.4%) | 1.452 → 1.554 (+7.0%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-structure.list | 1.326 → 1.532 (+15.6%) | 1.557 → 1.636 (+5.0%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.guardians | 1.093 → 1.147 (+5.0%) | 1.318 → 1.316 (-0.2%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.activities | 1.641 → 1.641 (+0.1%) | 1.896 → 1.782 (-6.0%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.categories | 0.681 → 0.801 (+17.6%) | 0.889 → 0.946 (+6.4%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.292 → 1.319 (+2.1%) | 1.506 → 1.407 (-6.6%) | 210.000 → 210.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| care-plan.offerings | 0.849 → 1.015 (+19.5%) | 1.098 → 1.182 (+7.7%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| communication.messages | 3.589 → 2.178 (-39.3%) | 4.014 → 2.384 (-40.6%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| appointments.calendar | 1.778 → 1.748 (-1.7%) | 1.954 → 1.852 (-5.2%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| settings.schema | 1.421 → 1.402 (-1.4%) | 1.857 → 1.748 (-5.9%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 0.806 → 0.922 (+14.4%) | 1.052 → 1.132 (+7.6%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 0.640 → 0.768 (+20.1%) | 0.886 → 0.863 (-2.5%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 2.656 → 2.788 (+4.9%) | 2.961 → 3.017 (+1.9%) | 450.000 → 450.000 (+0.0%) | 1170.000 → 1170.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.invalid-id | 0.480 → 0.582 (+21.2%) | 0.721 → 0.746 (+3.4%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.489 → 0.563 (+15.2%) | 0.695 → 0.586 (-15.7%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.481 → 0.562 (+16.7%) | 0.709 → 0.594 (-16.2%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.020 → 0.034 (+66.1%) | 0.057 → 0.044 (-22.2%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 4.711 → 5.877 (+24.7%) | 5.684 → 6.877 (+21.0%) | 1020.000 → 1020.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | job_duration_total_ms, latency_p50_ms, latency_p95_ms |
| delivery.render-failure | 4.383 → 4.391 (+0.2%) | 4.738 → 4.799 (+1.3%) | 960.000 → 960.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.idle | 2.784 → 2.842 (+2.1%) | 3.079 → 3.109 (+1.0%) | 600.000 → 600.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-create | 3.392 → 3.503 (+3.3%) | 3.779 → 3.833 (+1.4%) | 570.000 → 570.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-existing | 2.865 → 3.024 (+5.6%) | 3.098 → 3.263 (+5.3%) | 510.000 → 510.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 0.920 → 0.892 (-3.1%) | 1.445 → 1.142 (-21.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| organization-tenancy.resolve | 2.304 → 2.197 (-4.7%) | 2.899 → 3.146 (+8.5%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.update | 1.152 → 1.227 (+6.5%) | 1.575 → 1.652 (+4.9%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 1.340 → 1.462 (+9.1%) | 1.562 → 1.704 (+9.1%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-structure.list | 1.359 → 1.572 (+15.7%) | 1.629 → 2.505 (+53.8%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| people-directory.guardians | 1.182 → 1.158 (-2.0%) | 1.787 → 1.418 (-20.7%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.activities | 1.897 → 1.711 (-9.8%) | 2.095 → 1.928 (-8.0%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.categories | 0.761 → 0.835 (+9.7%) | 1.038 → 0.960 (-7.6%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.349 → 1.363 (+1.0%) | 1.648 → 1.440 (-12.6%) | 210.000 → 210.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| care-plan.offerings | 0.900 → 1.087 (+20.7%) | 1.178 → 1.251 (+6.2%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| communication.messages | 3.824 → 3.536 (-7.5%) | 4.929 → 3.888 (-21.1%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| appointments.calendar | 2.029 → 1.990 (-1.9%) | 2.365 → 2.383 (+0.7%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| settings.schema | 1.586 → 1.669 (+5.2%) | 2.097 → 1.958 (-6.6%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 1.021 → 1.058 (+3.6%) | 1.174 → 1.268 (+8.0%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 0.827 → 0.782 (-5.5%) | 1.034 → 0.903 (-12.7%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 2.668 → 2.842 (+6.5%) | 3.376 → 3.087 (-8.6%) | 450.000 → 450.000 (+0.0%) | 1170.000 → 1170.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.invalid-id | 0.482 → 0.611 (+26.9%) | 0.751 → 0.764 (+1.7%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.506 → 0.583 (+15.3%) | 0.832 → 0.741 (-11.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.498 → 0.578 (+16.1%) | 0.796 → 0.883 (+10.9%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.020 → 0.034 (+66.0%) | 0.058 → 0.045 (-23.2%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 4.774 → 5.968 (+25.0%) | 6.950 → 7.202 (+3.6%) | 1020.000 → 1020.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms |
| delivery.render-failure | 4.533 → 4.660 (+2.8%) | 5.650 → 6.005 (+6.3%) | 960.000 → 960.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.idle | 2.878 → 2.858 (-0.7%) | 3.097 → 3.193 (+3.1%) | 600.000 → 600.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-create | 3.621 → 3.692 (+2.0%) | 4.623 → 3.895 (-15.8%) | 570.000 → 570.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-existing | 2.871 → 3.042 (+6.0%) | 3.335 → 3.401 (+2.0%) | 510.000 → 510.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `school-structure.list` `latency_p95_ms` worst: 1.629 → 2.505
- `communication.messages` `latency_p50_ms` median: 3.589 → 2.178
- `communication.messages` `latency_p95_ms` median: 4.014 → 2.384
- `communication.messages` `latency_p95_ms` worst: 4.929 → 3.888
- `delivery.provider-unavailable` `latency_p50_ms` median: 4.711 → 5.877
- `delivery.provider-unavailable` `latency_p50_ms` worst: 4.774 → 5.968
- `delivery.provider-unavailable` `latency_p95_ms` median: 5.684 → 6.877
- `delivery.provider-unavailable` `job_duration_total_ms` median: 141.285 → 172.022

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 888
- within-tolerance: 102
- material: 8
- coverage: 100

## Scenarios only in the candidate workload

Not compared: `identity-access.login`, `identity-access.login-wrong-password`, `identity-access.refresh`, `identity-access.refresh-invalid-token`, `identity-access.mfa-verify`, `identity-access.mfa-verify-wrong-code`, `identity-access.passkey-login-options`, `identity-access.passkey-login-options-wrong-origin`, `device-scan.checkin`, `device-scan.checkin-invalid-key`, `device-scan.pickup-query`, `device-scan.pickup-query-missing-pin`, `device-scan.status`, `device-scan.status-wrong-pin`, `device-scan.rfid-lookup`, `device-scan.rfid-lookup-malformed-key`, `device-scan.staff-clock`, `device-scan.staff-clock-wrong-pin`, `people-directory.students`, `people-directory.students-invalid-view`, `request-review.queue`, `request-review.queue-invalid-view`, `request-review.care-schedule-decide`, `request-review.care-schedule-decide-not-found`, `care-plan.withdrawals`, `care-plan.withdrawals-invalid-state`, `student-deletion.withdrawal-impact`, `student-deletion.withdrawal-impact-not-found`, `care-plan.care-end-preview`, `care-plan.care-end-preview-past-day`, `care-plan.care-end`, `care-plan.care-end-replan`, `care-plan.care-end-stale-token`, `student-deletion.execute`, `student-deletion.execute-stale-preview`.
