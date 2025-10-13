# github.com/moto-nrw/project-phoenix

MOTO REST API for RFID-based system.

## Routes

<details>
<summary>`/`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/**
	- _GET_
		- [(*API).registerRoutesWithRateLimiting.func1]()

</details>
<details>
<summary>`/api/active/analytics/counts`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/analytics**
			- **/counts**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.6.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getCounts-fm]()

</details>
<details>
<summary>`/api/active/analytics/dashboard`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/analytics**
			- **/dashboard**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.6.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getDashboardAnalytics-fm]()

</details>
<details>
<summary>`/api/active/analytics/room/{roomId}/utilization`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/analytics**
			- **/room/{roomId}/utilization**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.6.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getRoomUtilization-fm]()

</details>
<details>
<summary>`/api/active/analytics/student/{studentId}/attendance`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/analytics**
			- **/student/{studentId}/attendance**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.6.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStudentAttendance-fm]()

</details>
<details>
<summary>`/api/active/combined`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/combined**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).listCombinedGroups-fm]()
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).createCombinedGroup-fm]()

</details>
<details>
<summary>`/api/active/combined/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/combined**
			- **/active**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveCombinedGroups-fm]()

</details>
<details>
<summary>`/api/active/combined/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/combined**
			- **/{id}**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).deleteCombinedGroup-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getCombinedGroup-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).updateCombinedGroup-fm]()

</details>
<details>
<summary>`/api/active/combined/{id}/end`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/combined**
			- **/{id}/end**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.8]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).endCombinedGroup-fm]()

</details>
<details>
<summary>`/api/active/combined/{id}/groups`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/combined**
			- **/{id}/groups**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.4.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getCombinedGroupGroups-fm]()

</details>
<details>
<summary>`/api/active/groups`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.9]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).createActiveGroup-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).listActiveGroups-fm]()

</details>
<details>
<summary>`/api/active/groups/group/{groupId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/group/{groupId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroupsByGroup-fm]()

</details>
<details>
<summary>`/api/active/groups/room/{roomId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/room/{roomId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroupsByRoom-fm]()

</details>
<details>
<summary>`/api/active/groups/unclaimed`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/unclaimed**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).listUnclaimedGroups-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.11]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).deleteActiveGroup-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroup-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.10]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).updateActiveGroup-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}/claim`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}/claim**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.13]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).claimGroup-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}/end`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}/end**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.12]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).endActiveGroup-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}/supervisors`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}/supervisors**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.8]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroupSupervisors-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}/visits`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}/visits**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroupVisits-fm]()

</details>
<details>
<summary>`/api/active/groups/{id}/visits/display`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/groups**
			- **/{id}/visits/display**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.1.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getActiveGroupVisitsWithDisplay-fm]()

</details>
<details>
<summary>`/api/active/mappings/add`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/mappings**
			- **/add**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.5.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).addGroupToCombination-fm]()

</details>
<details>
<summary>`/api/active/mappings/combined/{combinedId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/mappings**
			- **/combined/{combinedId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.5.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getCombinedGroupMappings-fm]()

</details>
<details>
<summary>`/api/active/mappings/group/{groupId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/mappings**
			- **/group/{groupId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.5.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getGroupMappings-fm]()

</details>
<details>
<summary>`/api/active/mappings/remove`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/mappings**
			- **/remove**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.5.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).removeGroupFromCombination-fm]()

</details>
<details>
<summary>`/api/active/scheduled-checkouts`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/scheduled-checkouts**
			- **/**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).createScheduledCheckout-fm]()

</details>
<details>
<summary>`/api/active/scheduled-checkouts/process`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/scheduled-checkouts**
			- **/process**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).processScheduledCheckouts-fm]()

</details>
<details>
<summary>`/api/active/scheduled-checkouts/student/{studentId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/scheduled-checkouts**
			- **/student/{studentId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStudentScheduledCheckouts-fm]()

</details>
<details>
<summary>`/api/active/scheduled-checkouts/student/{studentId}/pending`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/scheduled-checkouts**
			- **/student/{studentId}/pending**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getPendingScheduledCheckout-fm]()

</details>
<details>
<summary>`/api/active/scheduled-checkouts/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/scheduled-checkouts**
			- **/{id}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getScheduledCheckout-fm]()
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.7.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).cancelScheduledCheckout-fm]()

</details>
<details>
<summary>`/api/active/supervisors`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).createSupervisor-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).listSupervisors-fm]()

</details>
<details>
<summary>`/api/active/supervisors/group/{groupId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/group/{groupId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getSupervisorsByGroup-fm]()

</details>
<details>
<summary>`/api/active/supervisors/staff/{staffId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/staff/{staffId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStaffSupervisions-fm]()

</details>
<details>
<summary>`/api/active/supervisors/staff/{staffId}/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/staff/{staffId}/active**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStaffActiveSupervisions-fm]()

</details>
<details>
<summary>`/api/active/supervisors/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/{id}**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.8]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).deleteSupervisor-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getSupervisor-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).updateSupervisor-fm]()

</details>
<details>
<summary>`/api/active/supervisors/{id}/end`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/supervisors**
			- **/{id}/end**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.3.RequiresPermission.9]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).endSupervision-fm]()

</details>
<details>
<summary>`/api/active/visits`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).listVisits-fm]()
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).createVisit-fm]()

</details>
<details>
<summary>`/api/active/visits/group/{groupId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/group/{groupId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getVisitsByGroup-fm]()

</details>
<details>
<summary>`/api/active/visits/student/{studentId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/student/{studentId}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStudentVisits-fm]()

</details>
<details>
<summary>`/api/active/visits/student/{studentId}/checkout`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/student/{studentId}/checkout**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.11]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).checkoutStudent-fm]()

</details>
<details>
<summary>`/api/active/visits/student/{studentId}/current`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/student/{studentId}/current**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getStudentCurrentVisit-fm]()

</details>
<details>
<summary>`/api/active/visits/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/{id}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.(*ResourceAuthorizer).RequiresResourceAccess.3]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).getVisit-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.8]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).updateVisit-fm]()
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.9]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).deleteVisit-fm]()

</details>
<details>
<summary>`/api/active/visits/{id}/end`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/active**
		- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.SetContentType.func2]()
		- **/visits**
			- **/{id}/end**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/active.(*Resource).Router.func1.2.RequiresPermission.10]()
					- [oto-nrw/project-phoenix/api/active.(*Resource).endVisit-fm]()

</details>
<details>
<summary>`/api/activities`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).createActivity-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).listActivities-fm]()

</details>
<details>
<summary>`/api/activities/categories`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/categories**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).listCategories-fm]()

</details>
<details>
<summary>`/api/activities/quick-create`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/quick-create**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).quickCreateActivity-fm]()

</details>
<details>
<summary>`/api/activities/schedules/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/schedules/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getAvailableTimeSlots-fm]()

</details>
<details>
<summary>`/api/activities/students/{studentId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/students/{studentId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getStudentEnrollments-fm]()

</details>
<details>
<summary>`/api/activities/students/{studentId}/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/students/{studentId}/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getAvailableActivities-fm]()

</details>
<details>
<summary>`/api/activities/supervisors/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/supervisors/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getAvailableSupervisors-fm]()

</details>
<details>
<summary>`/api/activities/timespans`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/timespans**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getTimespans-fm]()

</details>
<details>
<summary>`/api/activities/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getActivity-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).updateActivity-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).deleteActivity-fm]()

</details>
<details>
<summary>`/api/activities/{id}/schedules`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/schedules**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getActivitySchedules-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).createActivitySchedule-fm]()

</details>
<details>
<summary>`/api/activities/{id}/schedules/{scheduleId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/schedules/{scheduleId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getActivitySchedule-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).updateActivitySchedule-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).deleteActivitySchedule-fm]()

</details>
<details>
<summary>`/api/activities/{id}/students`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/students**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).updateGroupEnrollments-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getActivityStudents-fm]()

</details>
<details>
<summary>`/api/activities/{id}/students/{studentId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/students/{studentId}**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).unenrollStudent-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).enrollStudent-fm]()

</details>
<details>
<summary>`/api/activities/{id}/supervisors`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/supervisors**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).getActivitySupervisors-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).assignSupervisor-fm]()

</details>
<details>
<summary>`/api/activities/{id}/supervisors/{supervisorId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/activities**
		- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.SetContentType.func2]()
		- **/{id}/supervisors/{supervisorId}**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).updateSupervisorRole-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/activities.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/activities.(*Resource).removeSupervisor-fm]()

</details>
<details>
<summary>`/api/config`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).listSettings-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).createSetting-fm]()

</details>
<details>
<summary>`/api/config/category/{category}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/category/{category}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getSettingsByCategory-fm]()

</details>
<details>
<summary>`/api/config/defaults`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/defaults**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getDefaultSettings-fm]()

</details>
<details>
<summary>`/api/config/import`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/import**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.11]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).importSettings-fm]()

</details>
<details>
<summary>`/api/config/initialize-defaults`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/initialize-defaults**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.12]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).initializeDefaults-fm]()

</details>
<details>
<summary>`/api/config/key/{key}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/key/{key}**
			- _PATCH_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).updateSettingValue-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getSettingByKey-fm]()

</details>
<details>
<summary>`/api/config/retention`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/retention**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.14]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).updateRetentionSettings-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.13]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getRetentionSettings-fm]()

</details>
<details>
<summary>`/api/config/retention/cleanup`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/retention/cleanup**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.15]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).triggerRetentionCleanup-fm]()

</details>
<details>
<summary>`/api/config/retention/stats`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/retention/stats**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.16]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getRetentionStats-fm]()

</details>
<details>
<summary>`/api/config/system-status`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/system-status**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getSystemStatus-fm]()

</details>
<details>
<summary>`/api/config/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/config**
		- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).getSetting-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).updateSetting-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.17]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/config.(*Resource).Router.func1.RequiresPermission.10]()
				- [oto-nrw/project-phoenix/api/config.(*Resource).deleteSetting-fm]()

</details>
<details>
<summary>`/api/database/stats`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/database**
		- [github.com/moto-nrw/project-phoenix/api/database.(*Resource).Router.SetContentType.func2]()
		- **/stats**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/database.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/database.(*Resource).getStats-fm]()

</details>
<details>
<summary>`/api/feedback`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).listFeedback-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).createFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/batch`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/batch**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).createBatchFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/date-range`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/date-range**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).getDateRangeFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/date/{date}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/date/{date}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).getDateFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/mensa`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/mensa**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).getMensaFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/student/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/student/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).getStudentFeedback-fm]()

</details>
<details>
<summary>`/api/feedback/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/feedback**
		- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).getFeedback-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/feedback.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/feedback.(*Resource).deleteFeedback-fm]()

</details>
<details>
<summary>`/api/groups`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).listGroups-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).createGroup-fm]()

</details>
<details>
<summary>`/api/groups/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).deleteGroup-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).getGroup-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).updateGroup-fm]()

</details>
<details>
<summary>`/api/groups/{id}/students`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/{id}/students**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).getGroupStudents-fm]()

</details>
<details>
<summary>`/api/groups/{id}/students/room-status`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/{id}/students/room-status**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).getGroupStudentsRoomStatus-fm]()

</details>
<details>
<summary>`/api/groups/{id}/substitutions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/{id}/substitutions**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).getGroupSubstitutions-fm]()

</details>
<details>
<summary>`/api/groups/{id}/supervisors`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/groups**
		- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.SetContentType.func2]()
		- **/{id}/supervisors**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/groups.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/groups.(*Resource).getGroupSupervisors-fm]()

</details>
<details>
<summary>`/api/iot`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).listDevices-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.11]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).createDevice-fm]()

</details>
<details>
<summary>`/api/iot/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/active**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getActiveDevices-fm]()

</details>
<details>
<summary>`/api/iot/activities`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/activities**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getTeacherActivities-fm]()

</details>
<details>
<summary>`/api/iot/attendance/status/{rfid}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/attendance/status/{rfid}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getAttendanceStatus-fm]()

</details>
<details>
<summary>`/api/iot/attendance/toggle`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/attendance/toggle**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).toggleAttendance-fm]()

</details>
<details>
<summary>`/api/iot/checkin`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/checkin**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).deviceCheckin-fm]()

</details>
<details>
<summary>`/api/iot/detect-new`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/detect-new**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.16]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).detectNewDevices-fm]()

</details>
<details>
<summary>`/api/iot/device/{deviceId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/device/{deviceId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDeviceByDeviceID-fm]()

</details>
<details>
<summary>`/api/iot/feedback`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/feedback**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).deviceSubmitFeedback-fm]()

</details>
<details>
<summary>`/api/iot/maintenance`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/maintenance**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDevicesRequiringMaintenance-fm]()

</details>
<details>
<summary>`/api/iot/offline`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/offline**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getOfflineDevices-fm]()

</details>
<details>
<summary>`/api/iot/ping`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/ping**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).devicePing-fm]()

</details>
<details>
<summary>`/api/iot/registered-by/{personId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/registered-by/{personId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDevicesByRegisteredBy-fm]()

</details>
<details>
<summary>`/api/iot/rfid/{tagId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/rfid/{tagId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).checkRFIDTagAssignment-fm]()

</details>
<details>
<summary>`/api/iot/rooms/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/rooms/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getAvailableRoomsForDevice-fm]()

</details>
<details>
<summary>`/api/iot/scan-network`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/scan-network**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.17]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).scanNetwork-fm]()

</details>
<details>
<summary>`/api/iot/session/activity`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/activity**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).updateSessionActivity-fm]()

</details>
<details>
<summary>`/api/iot/session/check-conflict`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/check-conflict**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).checkSessionConflict-fm]()

</details>
<details>
<summary>`/api/iot/session/current`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/current**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getCurrentSession-fm]()

</details>
<details>
<summary>`/api/iot/session/end`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/end**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).endActivitySession-fm]()

</details>
<details>
<summary>`/api/iot/session/start`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/start**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).startActivitySession-fm]()

</details>
<details>
<summary>`/api/iot/session/timeout`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/timeout**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).processSessionTimeout-fm]()

</details>
<details>
<summary>`/api/iot/session/timeout-config`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/timeout-config**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getSessionTimeoutConfig-fm]()

</details>
<details>
<summary>`/api/iot/session/timeout-info`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/timeout-info**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getSessionTimeoutInfo-fm]()

</details>
<details>
<summary>`/api/iot/session/validate-timeout`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/validate-timeout**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).validateSessionTimeout-fm]()

</details>
<details>
<summary>`/api/iot/session/{sessionId}/supervisors`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/session/{sessionId}/supervisors**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).updateSessionSupervisors-fm]()

</details>
<details>
<summary>`/api/iot/staff/{staffId}/rfid`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/staff/{staffId}/rfid**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).unassignStaffRFIDTag-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).assignStaffRFIDTag-fm]()

</details>
<details>
<summary>`/api/iot/statistics`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/statistics**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.10]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDeviceStatistics-fm]()

</details>
<details>
<summary>`/api/iot/status`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/status**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).deviceStatus-fm]()

</details>
<details>
<summary>`/api/iot/status/{status}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/status/{status}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDevicesByStatus-fm]()

</details>
<details>
<summary>`/api/iot/students`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/students**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func4.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getTeacherStudents-fm]()

</details>
<details>
<summary>`/api/iot/teachers`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/teachers**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func3.DeviceOnlyAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getAvailableTeachers-fm]()

</details>
<details>
<summary>`/api/iot/type/{type}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/type/{type}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDevicesByType-fm]()

</details>
<details>
<summary>`/api/iot/{deviceId}/ping`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/{deviceId}/ping**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.15]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).pingDevice-fm]()

</details>
<details>
<summary>`/api/iot/{deviceId}/status`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/{deviceId}/status**
			- _PATCH_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.14]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).updateDeviceStatus-fm]()

</details>
<details>
<summary>`/api/iot/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/iot**
		- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.SetContentType.func5]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).getDevice-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.12]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).updateDevice-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.18]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/iot.(*Resource).Router.func2.RequiresPermission.13]()
				- [oto-nrw/project-phoenix/api/iot.(*Resource).deleteDevice-fm]()

</details>
<details>
<summary>`/api/me`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/**
			- _GET_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getCurrentUser-fm]()

</details>
<details>
<summary>`/api/me/groups`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/**
				- _GET_
					- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getMyGroups-fm]()

</details>
<details>
<summary>`/api/me/groups/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/active**
				- _GET_
					- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getMyActiveGroups-fm]()

</details>
<details>
<summary>`/api/me/groups/activity`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/activity**
				- _GET_
					- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getMyActivityGroups-fm]()

</details>
<details>
<summary>`/api/me/groups/supervised`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/supervised**
				- _GET_
					- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getMySupervisedGroups-fm]()

</details>
<details>
<summary>`/api/me/groups/{groupID}/students`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/{groupID}**
				- **/students**
					- _GET_
						- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getGroupStudents-fm]()

</details>
<details>
<summary>`/api/me/groups/{groupID}/visits`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/groups**
			- **/{groupID}**
				- **/visits**
					- _GET_
						- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getGroupVisits-fm]()

</details>
<details>
<summary>`/api/me/profile`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/profile**
			- _PUT_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).updateCurrentProfile-fm]()
			- _GET_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getCurrentProfile-fm]()

</details>
<details>
<summary>`/api/me/profile/avatar`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/profile/avatar**
			- _POST_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).uploadAvatar-fm]()
			- _DELETE_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).deleteAvatar-fm]()

</details>
<details>
<summary>`/api/me/profile/avatar/{filename}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/profile/avatar/{filename}**
			- _GET_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).serveAvatar-fm]()

</details>
<details>
<summary>`/api/me/staff`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/staff**
			- _GET_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getCurrentStaff-fm]()

</details>
<details>
<summary>`/api/me/teacher`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/me**
		- [github.com/moto-nrw/project-phoenix/api/usercontext.NewResource.(*TokenAuth).Verifier.Verifier.Verify.func3]()
		- [Authenticator]()
		- **/teacher**
			- _GET_
				- [oto-nrw/project-phoenix/api/usercontext.(*Resource).getCurrentTeacher-fm]()

</details>
<details>
<summary>`/api/rooms`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/**
			- _GET_
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).listRooms-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).createRoom-fm]()

</details>
<details>
<summary>`/api/rooms/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getAvailableRooms-fm]()

</details>
<details>
<summary>`/api/rooms/buildings`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/buildings**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getBuildingList-fm]()

</details>
<details>
<summary>`/api/rooms/by-category`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/by-category**
			- _GET_
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getRoomsByCategory-fm]()

</details>
<details>
<summary>`/api/rooms/categories`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/categories**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getCategoryList-fm]()

</details>
<details>
<summary>`/api/rooms/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/{id}**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).deleteRoom-fm]()
			- _GET_
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getRoom-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.func2.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).updateRoom-fm]()

</details>
<details>
<summary>`/api/rooms/{id}/history`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/rooms**
		- [github.com/moto-nrw/project-phoenix/api/rooms.(*Resource).Router.SetContentType.func3]()
		- **/{id}/history**
			- _GET_
				- [oto-nrw/project-phoenix/api/rooms.(*Resource).getRoomHistory-fm]()

</details>
<details>
<summary>`/api/schedules/check-conflict`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/check-conflict**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.6]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/schedules.(*Resource).checkConflict-fm]()

</details>
<details>
<summary>`/api/schedules/current-dateframe`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/current-dateframe**
			- _GET_
				- [oto-nrw/project-phoenix/api/schedules.(*Resource).getCurrentDateframe-fm]()

</details>
<details>
<summary>`/api/schedules/dateframes`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/dateframes**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).listDateframes-fm]()
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).createDateframe-fm]()

</details>
<details>
<summary>`/api/schedules/dateframes/by-date`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/dateframes**
			- **/by-date**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getDateframesByDate-fm]()

</details>
<details>
<summary>`/api/schedules/dateframes/overlapping`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/dateframes**
			- **/overlapping**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getOverlappingDateframes-fm]()

</details>
<details>
<summary>`/api/schedules/dateframes/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/dateframes**
			- **/{id}**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).deleteDateframe-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getDateframe-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.1.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).updateDateframe-fm]()

</details>
<details>
<summary>`/api/schedules/find-available-slots`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/find-available-slots**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.(*TokenAuth).Verifier.Verifier.Verify.6]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/schedules.(*Resource).findAvailableSlots-fm]()

</details>
<details>
<summary>`/api/schedules/recurrence-rules`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/recurrence-rules**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).listRecurrenceRules-fm]()
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).createRecurrenceRule-fm]()

</details>
<details>
<summary>`/api/schedules/recurrence-rules/by-frequency`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/recurrence-rules**
			- **/by-frequency**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getRecurrenceRulesByFrequency-fm]()

</details>
<details>
<summary>`/api/schedules/recurrence-rules/by-weekday`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/recurrence-rules**
			- **/by-weekday**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getRecurrenceRulesByWeekday-fm]()

</details>
<details>
<summary>`/api/schedules/recurrence-rules/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/recurrence-rules**
			- **/{id}**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getRecurrenceRule-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).updateRecurrenceRule-fm]()
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).deleteRecurrenceRule-fm]()

</details>
<details>
<summary>`/api/schedules/recurrence-rules/{id}/generate-events`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/recurrence-rules**
			- **/{id}/generate-events**
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.3.RequiresPermission.8]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).generateEvents-fm]()

</details>
<details>
<summary>`/api/schedules/timeframes`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/timeframes**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).listTimeframes-fm]()
				- _POST_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).createTimeframe-fm]()

</details>
<details>
<summary>`/api/schedules/timeframes/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/timeframes**
			- **/active**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getActiveTimeframes-fm]()

</details>
<details>
<summary>`/api/schedules/timeframes/by-range`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/timeframes**
			- **/by-range**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.7]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getTimeframesByRange-fm]()

</details>
<details>
<summary>`/api/schedules/timeframes/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/schedules**
		- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.SetContentType.func3]()
		- **/timeframes**
			- **/{id}**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).deleteTimeframe-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).getTimeframe-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/schedules.(*Resource).Router.func2.2.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/schedules.(*Resource).updateTimeframe-fm]()

</details>
<details>
<summary>`/api/sse/events`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/sse**
		- **/events**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/sse.(*Resource).Router.func1.Verifier.Verify.1]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/sse.(*Resource).eventsHandler-fm]()

</details>
<details>
<summary>`/api/staff`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).createStaff-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).listStaff-fm]()

</details>
<details>
<summary>`/api/staff/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getAvailableStaff-fm]()

</details>
<details>
<summary>`/api/staff/available-for-substitution`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/available-for-substitution**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getAvailableForSubstitution-fm]()

</details>
<details>
<summary>`/api/staff/pin`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/pin**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).updatePIN-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getPINStatus-fm]()

</details>
<details>
<summary>`/api/staff/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).deleteStaff-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getStaff-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).updateStaff-fm]()

</details>
<details>
<summary>`/api/staff/{id}/groups`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/{id}/groups**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getStaffGroups-fm]()

</details>
<details>
<summary>`/api/staff/{id}/substitutions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/staff**
		- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.SetContentType.func2]()
		- **/{id}/substitutions**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.10]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/staff.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/staff.(*Resource).getStaffSubstitutions-fm]()

</details>
<details>
<summary>`/api/students`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).createStudent-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).listStudents-fm]()

</details>
<details>
<summary>`/api/students/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudent-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).updateStudent-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).deleteStudent-fm]()

</details>
<details>
<summary>`/api/students/{id}/current-location`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/current-location**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudentCurrentLocation-fm]()

</details>
<details>
<summary>`/api/students/{id}/current-visit`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/current-visit**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudentCurrentVisit-fm]()

</details>
<details>
<summary>`/api/students/{id}/in-group-room`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/in-group-room**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudentInGroupRoom-fm]()

</details>
<details>
<summary>`/api/students/{id}/privacy-consent`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/privacy-consent**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.10]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudentPrivacyConsent-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.11]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).updateStudentPrivacyConsent-fm]()

</details>
<details>
<summary>`/api/students/{id}/rfid`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/rfid**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func2.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).assignRFIDTag-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func2.DeviceAuthenticator.1]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).unassignRFIDTag-fm]()

</details>
<details>
<summary>`/api/students/{id}/visit-history`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/students**
		- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.SetContentType.func3]()
		- **/{id}/visit-history**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.12]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/students.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/students.(*Resource).getStudentVisitHistory-fm]()

</details>
<details>
<summary>`/api/substitutions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/substitutions**
		- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).list-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).create-fm]()

</details>
<details>
<summary>`/api/substitutions/active`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/substitutions**
		- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.SetContentType.func2]()
		- **/active**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).listActive-fm]()

</details>
<details>
<summary>`/api/substitutions/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/substitutions**
		- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).update-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).delete-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.7]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/substitutions.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/substitutions.(*Resource).get-fm]()

</details>
<details>
<summary>`/api/users`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.7]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).createPerson-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).listPersons-fm]()

</details>
<details>
<summary>`/api/users/by-account/{accountId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/by-account/{accountId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.5]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).getPersonByAccount-fm]()

</details>
<details>
<summary>`/api/users/by-tag/{tagId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/by-tag/{tagId}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).getPersonByTag-fm]()

</details>
<details>
<summary>`/api/users/rfid-cards/available`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/rfid-cards/available**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.6]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).listAvailableRFIDCards-fm]()

</details>
<details>
<summary>`/api/users/search`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/search**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.4]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).searchPersons-fm]()

</details>
<details>
<summary>`/api/users/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/{id}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).getPerson-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.8]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).updatePerson-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.9]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).deletePerson-fm]()

</details>
<details>
<summary>`/api/users/{id}/account`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/{id}/account**
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.12]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).linkAccount-fm]()
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.13]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).unlinkAccount-fm]()

</details>
<details>
<summary>`/api/users/{id}/profile`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/{id}/profile**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.14]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).getFullProfile-fm]()

</details>
<details>
<summary>`/api/users/{id}/rfid`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/api**
	- **/users**
		- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.SetContentType.func2]()
		- **/{id}/rfid**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.11]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).unlinkRFID-fm]()
			- _PUT_
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.15]()
				- [Authenticator]()
				- [github.com/moto-nrw/project-phoenix/api/users.(*Resource).Router.func1.RequiresPermission.10]()
				- [oto-nrw/project-phoenix/api/users.(*Resource).linkRFID-fm]()

</details>
<details>
<summary>`/auth/account`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/account**
		- _GET_
			- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.Verifier.Verify.2]()
			- [Authenticator]()
			- [oto-nrw/project-phoenix/api/auth.(*Resource).getAccount-fm]()

</details>
<details>
<summary>`/auth/accounts`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).listAccounts-fm]()

</details>
<details>
<summary>`/auth/accounts/by-role/{roleName}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/by-role/{roleName}**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).getAccountsByRole-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).updateAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/activate`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/activate**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.RequiresPermission.5]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).activateAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/deactivate`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/deactivate**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.RequiresPermission.6]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).deactivateAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/permissions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/permissions**
				- **/**
					- _GET_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.2.RequiresPermission.1]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).getAccountPermissions-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/permissions/direct`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/permissions**
				- **/direct**
					- _GET_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.2.RequiresPermission.2]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).getAccountDirectPermissions-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/permissions/{permissionId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/permissions**
				- **/{permissionId}**
					- _DELETE_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.2.RequiresPermission.5]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).removePermissionFromAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/permissions/{permissionId}/deny`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/permissions**
				- **/{permissionId}/deny**
					- _POST_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.2.RequiresPermission.4]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).denyPermissionToAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/permissions/{permissionId}/grant`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/permissions**
				- **/{permissionId}/grant**
					- _POST_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.2.RequiresPermission.3]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).grantPermissionToAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/roles`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/roles**
				- **/**
					- _GET_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.1.RequiresPermission.1]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).getAccountRoles-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/roles/{roleId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/roles**
				- **/{roleId}**
					- _POST_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.1.RequiresPermission.2]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).assignRoleToAccount-fm]()
					- _DELETE_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.1.RequiresPermission.3]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).removeRoleFromAccount-fm]()

</details>
<details>
<summary>`/auth/accounts/{accountId}/tokens`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/accounts**
		- **/{accountId}**
			- **/tokens**
				- **/**
					- _GET_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.3.RequiresPermission.1]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).getActiveTokens-fm]()
					- _DELETE_
						- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.3.1.3.RequiresPermission.2]()
						- [oto-nrw/project-phoenix/api/auth.(*Resource).revokeAllTokens-fm]()

</details>
<details>
<summary>`/auth/login`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/login**
		- _POST_
			- [oto-nrw/project-phoenix/api/auth.(*Resource).login-fm]()

</details>
<details>
<summary>`/auth/logout`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/logout**
		- _POST_
			- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func1.Verifier.Verify.1]()
			- [AuthenticateRefreshJWT]()
			- [oto-nrw/project-phoenix/api/auth.(*Resource).logout-fm]()

</details>
<details>
<summary>`/auth/parent-accounts`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/parent-accounts**
		- **/**
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).createParentAccount-fm]()
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).listParentAccounts-fm]()

</details>
<details>
<summary>`/auth/parent-accounts/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/parent-accounts**
		- **/{id}**
			- **/**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.1.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).getParentAccountByID-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.1.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).updateParentAccount-fm]()

</details>
<details>
<summary>`/auth/parent-accounts/{id}/activate`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/parent-accounts**
		- **/{id}**
			- **/activate**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.1.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).activateParentAccount-fm]()

</details>
<details>
<summary>`/auth/parent-accounts/{id}/deactivate`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/parent-accounts**
		- **/{id}**
			- **/deactivate**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.6.1.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).deactivateParentAccount-fm]()

</details>
<details>
<summary>`/auth/password`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/password**
		- _POST_
			- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.Verifier.Verify.2]()
			- [Authenticator]()
			- [oto-nrw/project-phoenix/api/auth.(*Resource).changePassword-fm]()

</details>
<details>
<summary>`/auth/password-reset`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/password-reset**
		- _POST_
			- [oto-nrw/project-phoenix/api/auth.(*Resource).initiatePasswordReset-fm]()

</details>
<details>
<summary>`/auth/password-reset/confirm`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/password-reset/confirm**
		- _POST_
			- [oto-nrw/project-phoenix/api/auth.(*Resource).resetPassword-fm]()

</details>
<details>
<summary>`/auth/permissions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/permissions**
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.2.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).listPermissions-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.2.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).createPermission-fm]()

</details>
<details>
<summary>`/auth/permissions/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/permissions**
		- **/{id}**
			- **/**
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.2.1.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).updatePermission-fm]()
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.2.1.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).deletePermission-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.2.1.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).getPermissionByID-fm]()

</details>
<details>
<summary>`/auth/refresh`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/refresh**
		- _POST_
			- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func1.Verifier.Verify.1]()
			- [AuthenticateRefreshJWT]()
			- [oto-nrw/project-phoenix/api/auth.(*Resource).refreshToken-fm]()

</details>
<details>
<summary>`/auth/register`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/register**
		- _POST_
			- [oto-nrw/project-phoenix/api/auth.(*Resource).register-fm]()

</details>
<details>
<summary>`/auth/roles`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/roles**
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).listRoles-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).createRole-fm]()

</details>
<details>
<summary>`/auth/roles/{id}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/roles**
		- **/{id}**
			- **/**
				- _DELETE_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.1.RequiresPermission.3]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).deleteRole-fm]()
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.1.RequiresPermission.1]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).getRoleByID-fm]()
				- _PUT_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.1.RequiresPermission.2]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).updateRole-fm]()

</details>
<details>
<summary>`/auth/roles/{id}/permissions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/roles**
		- **/{id}**
			- **/permissions**
				- _GET_
					- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.1.1.RequiresPermission.4]()
					- [oto-nrw/project-phoenix/api/auth.(*Resource).getRolePermissions-fm]()

</details>
<details>
<summary>`/auth/roles/{roleId}/permissions`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/roles/{roleId}/permissions**
		- **/**
			- _GET_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.4.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).getRolePermissions-fm]()

</details>
<details>
<summary>`/auth/roles/{roleId}/permissions/{permissionId}`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/roles/{roleId}/permissions**
		- **/{permissionId}**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.4.RequiresPermission.3]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).removePermissionFromRole-fm]()
			- _POST_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.4.RequiresPermission.2]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).assignPermissionToRole-fm]()

</details>
<details>
<summary>`/auth/tokens/expired`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.SetContentType.func3]()
	- **/tokens**
		- **/expired**
			- _DELETE_
				- [github.com/moto-nrw/project-phoenix/api/auth.(*Resource).Router.func2.1.5.RequiresPermission.1]()
				- [oto-nrw/project-phoenix/api/auth.(*Resource).cleanupExpiredTokens-fm]()

</details>
<details>
<summary>`/health`</summary>

- [RequestID]()
- [RealIP]()
- [Logger]()
- [Recoverer]()
- [SecurityHeaders]()
- **/health**
	- _GET_
		- [(*API).registerRoutesWithRateLimiting.func2]()

</details>

Total # of routes: 215
