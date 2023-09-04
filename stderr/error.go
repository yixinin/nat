package stderr

import (
	"fmt"
	"runtime/debug"
)

type Error struct {
	err    error
	stacks string
}

func (e Error) Error() string {
	return fmt.Sprintf("err:%v, stacks:%s", e.err, e.stacks)
}

func Wrap(err error) error {
	if err == nil {
		return err
	}
	return Error{
		err:    err,
		stacks: string(debug.Stack()[:]),
	}
}
