package driver

import "errors"

var (
	ErrProvisionerNameEmpty   = errors.New("provisioner name cannot be empty")
	ErrBucketHasLockedObjects = errors.New("bucket has objects with active object lock retention or legal hold")
)
