# github.com/moto-nrw/project-phoenix

MOTO REST API for RFID-based system.

## Routes

<details>
<summary>``</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- ****
	- _GET_
		- [New.SPAHandler.func5]()

</details>
<details>
<summary>`/activities`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/**
			- _POST_
				- [hax/go-base/api/activity.(*Resource).createActivityGroup-fm]()
			- _GET_
				- [hax/go-base/api/activity.(*Resource).listActivityGroups-fm]()

</details>
<details>
<summary>`/activities/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/{id}**
			- **/**
				- _PUT_
					- [hax/go-base/api/activity.(*Resource).updateActivityGroup-fm]()
				- _DELETE_
					- [hax/go-base/api/activity.(*Resource).deleteActivityGroup-fm]()
				- _GET_
					- [hax/go-base/api/activity.(*Resource).getActivityGroup-fm]()

</details>
<details>
<summary>`/activities/{id}/students`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/{id}**
			- **/students**
				- **/**
					- _GET_
						- [hax/go-base/api/activity.(*Resource).listEnrolledStudents-fm]()

</details>
<details>
<summary>`/activities/{id}/students/{studentId}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/{id}**
			- **/students**
				- **/{studentId}**
					- _DELETE_
						- [hax/go-base/api/activity.(*Resource).unenrollStudent-fm]()
					- _POST_
						- [hax/go-base/api/activity.(*Resource).enrollStudent-fm]()

</details>
<details>
<summary>`/activities/{id}/times`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/{id}**
			- **/times**
				- **/**
					- _GET_
						- [hax/go-base/api/activity.(*Resource).listAgTimes-fm]()
					- _POST_
						- [hax/go-base/api/activity.(*Resource).createAgTime-fm]()

</details>
<details>
<summary>`/activities/{id}/times/{timeId}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- ****
		- **/{id}**
			- **/times**
				- **/{timeId}**
					- **/**
						- _PUT_
							- [hax/go-base/api/activity.(*Resource).updateAgTime-fm]()
						- _DELETE_
							- [hax/go-base/api/activity.(*Resource).deleteAgTime-fm]()

</details>
<details>
<summary>`/activities/categories`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- **/categories**
		- **/**
			- _POST_
				- [hax/go-base/api/activity.(*Resource).createCategory-fm]()
			- _GET_
				- [hax/go-base/api/activity.(*Resource).listCategories-fm]()

</details>
<details>
<summary>`/activities/categories/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- **/categories**
		- **/{id}**
			- **/**
				- _PUT_
					- [hax/go-base/api/activity.(*Resource).updateCategory-fm]()
				- _DELETE_
					- [hax/go-base/api/activity.(*Resource).deleteCategory-fm]()
				- _GET_
					- [hax/go-base/api/activity.(*Resource).getCategory-fm]()

</details>
<details>
<summary>`/activities/student/{studentId}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/activities**
	- **/student/{studentId}**
		- _GET_
			- [Authenticator]()
			- [hax/go-base/api/activity.(*Resource).listStudentAgs-fm]()

</details>
<details>
<summary>`/admin`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/admin**
	- [github.com/moto-nrw/project-phoenix/api/admin.(*API).Router.RequiresRole.func2]()
	- **/**
		- _GET_
			- [(*API).Router.func1]()

</details>
<details>
<summary>`/admin/accounts`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/admin**
	- [github.com/moto-nrw/project-phoenix/api/admin.(*API).Router.RequiresRole.func2]()
	- **/accounts**
		- **/**
			- _GET_
				- [hax/go-base/api/admin.(*AccountResource).list-fm]()
			- _POST_
				- [hax/go-base/api/admin.(*AccountResource).create-fm]()

</details>
<details>
<summary>`/admin/accounts/{accountID}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/admin**
	- [github.com/moto-nrw/project-phoenix/api/admin.(*API).Router.RequiresRole.func2]()
	- **/accounts**
		- **/{accountID}**
			- [hax/go-base/api/admin.(*AccountResource).accountCtx-fm]()
			- **/**
				- _GET_
					- [hax/go-base/api/admin.(*AccountResource).get-fm]()
				- _PUT_
					- [hax/go-base/api/admin.(*AccountResource).update-fm]()
				- _DELETE_
					- [hax/go-base/api/admin.(*AccountResource).delete-fm]()

</details>
<details>
<summary>`/api/account`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/api**
	- **/account**
		- [hax/go-base/api/app.(*AccountResource).accountCtx-fm]()
		- **/**
			- _PUT_
				- [hax/go-base/api/app.(*AccountResource).update-fm]()
			- _DELETE_
				- [hax/go-base/api/app.(*AccountResource).delete-fm]()
			- _GET_
				- [hax/go-base/api/app.(*AccountResource).get-fm]()

</details>
<details>
<summary>`/api/account/token/{tokenID}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/api**
	- **/account**
		- [hax/go-base/api/app.(*AccountResource).accountCtx-fm]()
		- **/token/{tokenID}**
			- **/**
				- _DELETE_
					- [hax/go-base/api/app.(*AccountResource).deleteToken-fm]()
				- _PUT_
					- [hax/go-base/api/app.(*AccountResource).updateToken-fm]()

</details>
<details>
<summary>`/api/profile`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/api**
	- **/profile**
		- [hax/go-base/api/app.(*ProfileResource).profileCtx-fm]()
		- **/**
			- _GET_
				- [hax/go-base/api/app.(*ProfileResource).get-fm]()
			- _PUT_
				- [hax/go-base/api/app.(*ProfileResource).update-fm]()

</details>
<details>
<summary>`/auth/login`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.SetContentType.func2]()
	- **/login**
		- _POST_
			- [hax/go-base/auth/pwdless.(*Resource).login-fm]()

</details>
<details>
<summary>`/auth/logout`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.SetContentType.func2]()
	- **/logout**
		- _POST_
			- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
			- [AuthenticateRefreshJWT]()
			- [hax/go-base/auth/pwdless.(*Resource).logout-fm]()

</details>
<details>
<summary>`/auth/refresh`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.SetContentType.func2]()
	- **/refresh**
		- _POST_
			- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.func1.(*TokenAuth).Verifier.Verifier.Verify.1]()
			- [AuthenticateRefreshJWT]()
			- [hax/go-base/auth/pwdless.(*Resource).refresh-fm]()

</details>
<details>
<summary>`/auth/token`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/auth**
	- [github.com/moto-nrw/project-phoenix/auth/pwdless.(*Resource).Router.SetContentType.func2]()
	- **/token**
		- _POST_
			- [hax/go-base/auth/pwdless.(*Resource).token-fm]()

</details>
<details>
<summary>`/groups`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- ****
		- **/**
			- _GET_
				- [hax/go-base/api/group.(*Resource).listGroups-fm]()
			- _POST_
				- [hax/go-base/api/group.(*Resource).createGroup-fm]()

</details>
<details>
<summary>`/groups/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- ****
		- **/{id}**
			- **/**
				- _PUT_
					- [hax/go-base/api/group.(*Resource).updateGroup-fm]()
				- _DELETE_
					- [hax/go-base/api/group.(*Resource).deleteGroup-fm]()
				- _GET_
					- [hax/go-base/api/group.(*Resource).getGroup-fm]()

</details>
<details>
<summary>`/groups/{id}/supervisors`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- ****
		- **/{id}**
			- **/supervisors**
				- _POST_
					- [hax/go-base/api/group.(*Resource).updateGroupSupervisors-fm]()

</details>
<details>
<summary>`/groups/combined`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- **/combined**
		- **/**
			- _GET_
				- [hax/go-base/api/group.(*Resource).listCombinedGroups-fm]()
			- _POST_
				- [hax/go-base/api/group.(*Resource).createCombinedGroup-fm]()

</details>
<details>
<summary>`/groups/combined/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- **/combined**
		- **/{id}**
			- **/**
				- _GET_
					- [hax/go-base/api/group.(*Resource).getCombinedGroup-fm]()

</details>
<details>
<summary>`/groups/merge-rooms`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/groups**
	- **/merge-rooms**
		- _POST_
			- [Authenticator]()
			- [hax/go-base/api/group.(*Resource).mergeRooms-fm]()

</details>
<details>
<summary>`/healthz`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/healthz**
	- _GET_
		- [New.func2]()

</details>
<details>
<summary>`/rfid/app/status`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/app/status**
		- _GET_
			- [hax/go-base/api/rfid.(*API).handleTauriStatus-fm]()

</details>
<details>
<summary>`/rfid/app/sync`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/app/sync**
		- _POST_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleTauriSync-fm]()

</details>
<details>
<summary>`/rfid/devices`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/devices**
		- **/**
			- _GET_
				- [hax/go-base/api/rfid.(*API).handleListDevices-fm]()

</details>
<details>
<summary>`/rfid/devices/{device_id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/devices**
		- **/{device_id}**
			- _GET_
				- [hax/go-base/api/rfid.(*API).handleGetDevice-fm]()
			- _PUT_
				- [hax/go-base/api/rfid.(*API).handleUpdateDevice-fm]()

</details>
<details>
<summary>`/rfid/devices/{device_id}/sync-history`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/devices**
		- **/{device_id}/sync-history**
			- _GET_
				- [hax/go-base/api/rfid.(*API).handleGetDeviceSyncHistory-fm]()

</details>
<details>
<summary>`/rfid/room-entry`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/room-entry**
		- _POST_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleRoomEntry-fm]()

</details>
<details>
<summary>`/rfid/room-exit`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/room-exit**
		- _POST_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleRoomExit-fm]()

</details>
<details>
<summary>`/rfid/room-occupancy`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/room-occupancy**
		- _GET_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleGetRoomOccupancy-fm]()

</details>
<details>
<summary>`/rfid/room/{id}/visits`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/room/{id}/visits**
		- _GET_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleGetRoomVisits-fm]()

</details>
<details>
<summary>`/rfid/student/{id}/visits`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/student/{id}/visits**
		- _GET_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleGetStudentVisits-fm]()

</details>
<details>
<summary>`/rfid/tag`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/tag**
		- _POST_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleTagRead-fm]()

</details>
<details>
<summary>`/rfid/tags`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/tags**
		- _GET_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleGetAllTags-fm]()

</details>
<details>
<summary>`/rfid/track-student`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/track-student**
		- _POST_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleStudentTracking-fm]()

</details>
<details>
<summary>`/rfid/visits/today`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rfid**
	- **/visits/today**
		- _GET_
			- [hax/go-base/api/rfid.(*API).apiKeyAuthMiddleware-fm]()
			- [hax/go-base/api/rfid.(*API).handleGetTodayVisits-fm]()

</details>
<details>
<summary>`/rooms`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/**
		- _POST_
			- [hax/go-base/api/room.(*API).handleCreateRoom-fm]()
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetRooms-fm]()

</details>
<details>
<summary>`/rooms/choose`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/choose**
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetRoomsForSelection-fm]()

</details>
<details>
<summary>`/rooms/combined_groups`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/combined_groups**
		- **/**
			- _GET_
				- [hax/go-base/api/room.(*API).handleGetActiveCombinedGroups-fm]()

</details>
<details>
<summary>`/rooms/combined_groups/merge`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/combined_groups**
		- **/merge**
			- _POST_
				- [hax/go-base/api/room.(*API).handleMergeRooms-fm]()

</details>
<details>
<summary>`/rooms/combined_groups/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/combined_groups**
		- **/{id}**
			- _DELETE_
				- [hax/go-base/api/room.(*API).handleDeactivateCombinedGroup-fm]()

</details>
<details>
<summary>`/rooms/grouped_by_category`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/grouped_by_category**
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetRoomsGroupedByCategory-fm]()

</details>
<details>
<summary>`/rooms/occupancies`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/occupancies**
		- **/**
			- _GET_
				- [hax/go-base/api/room.(*API).handleGetAllRoomOccupancies-fm]()

</details>
<details>
<summary>`/rooms/occupancies/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/occupancies**
		- **/{id}**
			- _GET_
				- [hax/go-base/api/room.(*API).handleGetRoomOccupancyByID-fm]()

</details>
<details>
<summary>`/rooms/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/{id}**
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetRoomByID-fm]()
		- _PUT_
			- [hax/go-base/api/room.(*API).handleUpdateRoom-fm]()
		- _DELETE_
			- [hax/go-base/api/room.(*API).handleDeleteRoom-fm]()

</details>
<details>
<summary>`/rooms/{id}/combined_group`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/{id}/combined_group**
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetCombinedGroupForRoom-fm]()

</details>
<details>
<summary>`/rooms/{id}/current_occupancy`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/{id}/current_occupancy**
		- _GET_
			- [hax/go-base/api/room.(*API).handleGetCurrentRoomOccupancy-fm]()

</details>
<details>
<summary>`/rooms/{id}/register_tablet`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/{id}/register_tablet**
		- _POST_
			- [hax/go-base/api/room.(*API).handleRegisterTablet-fm]()

</details>
<details>
<summary>`/rooms/{id}/unregister_tablet`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/rooms**
	- **/{id}/unregister_tablet**
		- _POST_
			- [hax/go-base/api/room.(*API).handleUnregisterTablet-fm]()

</details>
<details>
<summary>`/settings`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/settings**
	- **/**
		- _POST_
			- [hax/go-base/api/settings.(*Resource).Create-fm]()
		- _GET_
			- [hax/go-base/api/settings.(*Resource).List-fm]()

</details>
<details>
<summary>`/settings/category/{category}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/settings**
	- **/category/{category}**
		- _GET_
			- [hax/go-base/api/settings.(*Resource).GetByCategory-fm]()

</details>
<details>
<summary>`/settings/key/{key}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/settings**
	- **/key/{key}**
		- _GET_
			- [hax/go-base/api/settings.(*Resource).GetByKey-fm]()

</details>
<details>
<summary>`/settings/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/settings**
	- **/{id}**
		- _PUT_
			- [hax/go-base/api/settings.(*Resource).Update-fm]()
		- _DELETE_
			- [hax/go-base/api/settings.(*Resource).Delete-fm]()
		- _GET_
			- [hax/go-base/api/settings.(*Resource).Get-fm]()

</details>
<details>
<summary>`/settings/{key}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/settings**
	- **/{key}**
		- _PATCH_
			- [hax/go-base/api/settings.(*Resource).UpdateByKey-fm]()

</details>
<details>
<summary>`/students`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- ****
		- **/**
			- _GET_
				- [hax/go-base/api/student.(*Resource).listStudents-fm]()
			- _POST_
				- [hax/go-base/api/student.(*Resource).createStudent-fm]()

</details>
<details>
<summary>`/students/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- ****
		- **/{id}**
			- **/**
				- _DELETE_
					- [hax/go-base/api/student.(*Resource).deleteStudent-fm]()
				- _GET_
					- [hax/go-base/api/student.(*Resource).getStudent-fm]()
				- _PUT_
					- [hax/go-base/api/student.(*Resource).updateStudent-fm]()

</details>
<details>
<summary>`/students/{id}/visits`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- ****
		- **/{id}**
			- **/visits**
				- _GET_
					- [hax/go-base/api/student.(*Resource).getStudentVisits-fm]()

</details>
<details>
<summary>`/students/combined-group/{id}/visits`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- **/combined-group/{id}/visits**
		- _GET_
			- [Authenticator]()
			- [hax/go-base/api/student.(*Resource).getCombinedGroupVisits-fm]()

</details>
<details>
<summary>`/students/give-feedback`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- **/give-feedback**
		- _POST_
			- [Authenticator]()
			- [hax/go-base/api/student.(*Resource).giveFeedback-fm]()

</details>
<details>
<summary>`/students/register-in-room`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- **/register-in-room**
		- _POST_
			- [Authenticator]()
			- [hax/go-base/api/student.(*Resource).registerStudentInRoom-fm]()

</details>
<details>
<summary>`/students/unregister-from-room`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- **/unregister-from-room**
		- _POST_
			- [Authenticator]()
			- [hax/go-base/api/student.(*Resource).unregisterStudentFromRoom-fm]()

</details>
<details>
<summary>`/students/update-location`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/students**
	- **/update-location**
		- _POST_
			- [Authenticator]()
			- [hax/go-base/api/student.(*Resource).updateStudentLocation-fm]()

</details>
<details>
<summary>`/users/change-tag-id`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/change-tag-id**
		- _POST_
			- [Authenticator]()
			- [github.com/moto-nrw/project-phoenix/api/user.(*Resource).Router.func2.RequiresRole.4]()
			- [hax/go-base/api/user.(*Resource).changeTagID-fm]()

</details>
<details>
<summary>`/users/devices`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/devices**
		- **/**
			- _POST_
				- [hax/go-base/api/user.(*Resource).createDevice-fm]()

</details>
<details>
<summary>`/users/devices/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/devices**
		- **/{id}**
			- **/**
				- _DELETE_
					- [hax/go-base/api/user.(*Resource).deleteDevice-fm]()
				- _GET_
					- [hax/go-base/api/user.(*Resource).getDevice-fm]()

</details>
<details>
<summary>`/users/process-tag-scan`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/process-tag-scan**
		- _POST_
			- [Authenticator]()
			- [github.com/moto-nrw/project-phoenix/api/user.(*Resource).Router.func2.RequiresRole.4]()
			- [hax/go-base/api/user.(*Resource).processTagScan-fm]()

</details>
<details>
<summary>`/users/public/supervisors`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/public**
		- **/supervisors**
			- _GET_
				- [hax/go-base/api/user.(*Resource).listSupervisorsPublic-fm]()

</details>
<details>
<summary>`/users/public/users`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/public**
		- **/users**
			- _GET_
				- [hax/go-base/api/user.(*Resource).listUsersPublic-fm]()

</details>
<details>
<summary>`/users/specialists`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/specialists**
		- **/**
			- _GET_
				- [hax/go-base/api/user.(*Resource).listSpecialists-fm]()
			- _POST_
				- [hax/go-base/api/user.(*Resource).createSpecialist-fm]()

</details>
<details>
<summary>`/users/specialists/without-supervision`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/specialists**
		- **/without-supervision**
			- _GET_
				- [hax/go-base/api/user.(*Resource).listSpecialistsWithoutSupervision-fm]()

</details>
<details>
<summary>`/users/specialists/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/specialists**
		- **/{id}**
			- **/**
				- _PUT_
					- [hax/go-base/api/user.(*Resource).updateSpecialist-fm]()
				- _DELETE_
					- [hax/go-base/api/user.(*Resource).deleteSpecialist-fm]()
				- _GET_
					- [hax/go-base/api/user.(*Resource).getSpecialist-fm]()

</details>
<details>
<summary>`/users/users`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/users**
		- **/**
			- _POST_
				- [hax/go-base/api/user.(*Resource).createUser-fm]()
			- _GET_
				- [hax/go-base/api/user.(*Resource).listUsers-fm]()

</details>
<details>
<summary>`/users/users/{id}`</summary>

- [Recoverer]()
- [RequestID]()
- [github.com/moto-nrw/project-phoenix/api.New.Timeout.func3]()
- [github.com/moto-nrw/project-phoenix/api.New.NewStructuredLogger.RequestLogger.func7]()
- [github.com/moto-nrw/project-phoenix/api.New.SetContentType.func4]()
- **/users**
	- **/users**
		- **/{id}**
			- **/**
				- _PUT_
					- [hax/go-base/api/user.(*Resource).updateUser-fm]()
				- _DELETE_
					- [hax/go-base/api/user.(*Resource).deleteUser-fm]()
				- _GET_
					- [hax/go-base/api/user.(*Resource).getUser-fm]()

</details>

Total # of routes: 78
