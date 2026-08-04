package dots

import (
	"context"
)

func CanDoAnything(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.ID.IsNil() {
		return Errorf(EUNAUTHORIZED, "unauthorized user")
	}

	canDoAny := PowersContains(user.Powers, DoAnything)
	if canDoAny {
		return nil
	}

	return Errorf(EUNAUTHORIZED, "unauthorized operation")
}

func CanDeleteOwn(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.ID.IsNil() {
		return Errorf(EUNAUTHORIZED, "unauthorized user")
	}

	canDeleteOwn := PowersContains(user.Powers, DeleteOwn)
	if !canDeleteOwn {
		return Errorf(EUNAUTHORIZED, "unauthorized operation")
	}

	return nil
}

func CanWriteOwn(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.ID.IsNil() {
		return Errorf(EUNAUTHORIZED, "unauthorized user")
	}

	canWriteOwn := PowersContains(user.Powers, WriteOwn)
	if !canWriteOwn {
		return Errorf(EUNAUTHORIZED, "unauthorized operation")
	}

	return nil
}

func CanReadOwn(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.ID.IsNil() {
		return Errorf(EUNAUTHORIZED, "unauthorized user")
	}

	canReadOwn := PowersContains(user.Powers, ReadOwn)
	if !canReadOwn {
		return Errorf(EUNAUTHORIZED, "unauthorized operation")
	}

	return nil
}

func CanCreateOwn(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.ID.IsNil() {
		return Errorf(EUNAUTHORIZED, "unauthorized user")
	}

	canCreateOwn := PowersContains(user.Powers, CreateOwn)
	if !canCreateOwn {
		return Errorf(EUNAUTHORIZED, "unauthorized operation")
	}

	return nil
}
