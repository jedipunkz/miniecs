package ecs

import (
	"fmt"
)

// ErrExecuteCommand occurs when ecs:ExecuteCommand fails.
type ErrExecuteCommand struct {
	err error
}

// ErrGetTask is
type ErrGetTask struct {
	err error
}

// ErrListClusters is
type ErrListClusters struct {
	err error
}

// ErrListServices is
type ErrListServices struct {
	err error
}

// Error is printing execute command err
func (e *ErrExecuteCommand) Error() string {
	return fmt.Sprintf("execute command: %s", e.err.Error())
}

// Error is printing get task command err
func (e *ErrGetTask) Error() string {
	return fmt.Sprintf("get task command: %s", e.err.Error())
}

// Error is printing list clusters command err
func (e *ErrListClusters) Error() string {
	return fmt.Sprintf("list clusters command: %s", e.err.Error())
}

// Error is printing list services command err
func (e *ErrListServices) Error() string {
	return fmt.Sprintf("list services command: %s", e.err.Error())
}
