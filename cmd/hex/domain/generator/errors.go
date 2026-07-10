package generator

import hexerrors "github.com/jordanbrauer/hex/errors"

// Domain errors for Generator. Compare with errors.Is:
//
//	if errors.Is(err, generator.ErrBlueprintNotFound) { ... }
var (
	ErrBlueprintNotFound = hexerrors.New(hexerrors.CodeNotFound, "blueprint not found")
	ErrTargetExists      = hexerrors.New(hexerrors.CodeConflict, "target file already exists")
)
