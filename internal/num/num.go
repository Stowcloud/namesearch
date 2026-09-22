package num

import (
	"errors"
	"fmt"
)

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

var ErrNarrow = errors.New("num: value out of range for target type")

type RangeError struct{ Value, From, To string }

func (e *RangeError) Error() string {
	return fmt.Sprintf("num: %s (%s) out of range for %s", e.Value, e.From, e.To)
}
func (e *RangeError) Is(target error) bool { return target == ErrNarrow }
func Narrow[To, From Integer](v From) (To, error) {
	out := To(v)
	if From(out) != v || (v < 0) != (out < 0) {
		var z To
		return z, &RangeError{fmt.Sprintf("%d", v), fmt.Sprintf("%T", v), fmt.Sprintf("%T", out)}
	}
	return out, nil
}
