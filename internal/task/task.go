package task

import "context"

func Go(_ context.Context, _ string, fn func()) { go fn() }
