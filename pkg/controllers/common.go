package controllers

const (
	workFieldManagerName = "work-api-agent"

	eventReasonAppliedWorkCreated                  = "AppliedWorkCreated"
	eventReasonAppliedWorkDeleted                  = "AppliedWorkDeleted"
	eventReasonFinalizerAdded                      = "FinalizerAdded"
	eventReasonFinalizerRemoved                    = "FinalizerRemoved"
	eventReasonManifestApplyFailed                 = "ManifestApplyFailed"
	eventReasonManifestApplySucceeded              = "ManifestApplySucceeded"
	eventReasonReconcileIncomplete                 = "ReconciliationIncomplete"
	eventReasonResourceCreateSucceeded             = "ResourceCreated"
	eventReasonResourceGarbageCollectionComplete   = "ResourceGarbageCollectionComplete"
	eventReasonResourceGarbageCollectionIncomplete = "ResourceGarbageCollectionIncomplete"
	eventReasonResourceNotFound                    = "ResourceNotFound"
	eventReasonResourceNotOwnedByWorkAPI           = "ResourceNotOwnedByWorkAPI"
	eventReasonResourceStatusUpdateFailed          = "ResourceStatusUpdateFailed"
	eventReasonResourceUpdateStatusSucceeded       = "ResourceStatusUpdateSucceeded"
	eventReasonResourcePatchFailed                 = "ResourcePatchFailed"
	eventReasonResourcePatchSucceeded              = "ResourcePatchedSucceeded"
	eventResourceUpdateFailed                      = "ResourceUpdateFailed"
	eventReasonResourceUpdateSucceeded             = "ResourceUpdateSucceeded"

	messageManifestApplyFailed                 = "manifest apply failed"
	messageManifestApplyIncomplete             = "manifest apply incomplete; the respective Work will be queued again for reconciliation"
	messageManifestApplySucceeded              = "manifest apply succeeded"
	messageManifestApplyUnwarranted            = "manifest apply unwarranted; the spec has not changed"
	messageResourceFinalizerRemoved            = "resource's finalizer removed"
	messageResourceCreateSucceeded             = "resource create succeeded"
	messageResourceCreateFailed                = "resource create failed"
	messageResourceDeleteSucceeded             = "resource delete succeeded"
	messageResourceDeleting                    = "resource is in the process of being deleted"
	messageResourceDeleteFailed                = "resource delete failed"
	messageResourceDiscovered                  = "resource discovered"
	messageResourceFinalizerAdded              = "resource finalizer added"
	messageResourceGarbageCollectionComplete   = "resource garbage-collection complete"
	messageResourceGarbageCollectionIncomplete = "resource garbage-collection incomplete; some Work owned resources could not be deleted"
	messageResourceIsOrphan                    = "resource is an orphan"
	messageResourceIsMissingCondition          = "resource is missing condition"
	messageResourceJSONMarshalFailed           = "resource JSON marshaling failed"
	messageResourceNotOwnedByWorkAPI           = "resource not owned by Work-API"
	messageResourcePatchFailed                 = "resource patch failed"
	messageResourcePatchSucceeded              = "resource patch succeeded"
	messageResourceRetrieveFailed              = "resource retrieval failed"
	messageResourceStateInvalid                = "resource state is invalid"
	messageResourceSpecModified                = "resource spec modified"
	messageResourceStatusUpdateFailed          = "resource status update failed"
	messageResourceStatusUpdateSucceeded       = "resource status update succeeded"
	messageResourceUpdateFailed                = "resource update failed"
	messageResourceUpdateSucceeded             = "resource update succeeded"
)
