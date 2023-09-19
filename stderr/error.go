package stderr

import (
	"fmt"
	"runtime/debug"
	"strings"
)

type Error struct {
	code   string
	msg    string
	err    error
	stacks string
}

func (e Error) Error() string {
	var ss = make([]string, 0, 4)
	if e.code != "" {
		ss = append(ss, e.code)
	}
	if e.msg != "" {
		ss = append(ss, e.msg)
	}
	if e.err != nil {
		ss = append(ss, fmt.Sprintf("err:%s", e.err))
	}
	if e.stacks != "" {
		ss = append(ss, e.stacks)
	}
	return strings.Join(ss, ", ")
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

func New(code string, msg string, err ...error) error {
	e := Error{
		code:   code,
		msg:    msg,
		stacks: string(debug.Stack()[:]),
	}
	if len(err) > 0 {
		e.err = err[0]
	}
	return e
}
