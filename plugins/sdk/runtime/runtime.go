// Package runtime provides the main dispatch loop and lifecycle management for SDK plugins.
package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/errors"
)

// Handler processes IPC commands and returns results.
type Handler interface {
	// Handle processes a command and returns a result or error.
	Handle(cmd string, args map[string]interface{}) (interface{}, error)
}

// HandlerFunc is a function type that implements Handler.
type HandlerFunc func(cmd string, args map[string]interface{}) (interface{}, error)

// Handle implements Handler.
func (f HandlerFunc) Handle(cmd string, args map[string]interface{}) (interface{}, error) {
	return f(cmd, args)
}

// Run starts the main IPC loop, reading requests from stdin and writing responses to stdout.
func Run(handler Handler) error {
	return RunWithIO(handler, os.Stdin, os.Stdout)
}

// RunWithIO runs the IPC loop with custom input/output.
// This is primarily for testing.
func RunWithIO(handler Handler, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		response := processRequest(handler, line)
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("failed to encode response: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	return nil
}

// processRequest handles a single request and returns a response.
func processRequest(handler Handler, data []byte) *ipc.Response {
	// Parse request
	var req ipc.Request
	if err := json.Unmarshal(data, &req); err != nil {
		return errorResponse(errors.New(errors.CodeInvalidJSON, "failed to parse request"))
	}

	// Dispatch to handler
	result, err := handler.Handle(req.Command, req.Args)
	if err != nil {
		return errorResponse(err)
	}

	return &ipc.Response{
		Status: "ok",
		Result: result,
	}
}

// errorResponse creates an error response from an error.
func errorResponse(err error) *ipc.Response {
	return &ipc.Response{
		Status: "error",
		Error:  errors.ToMessage(err),
	}
}

// Dispatcher maps commands to handler functions.
type Dispatcher struct {
	handlers map[string]func(map[string]interface{}) (interface{}, error)
}

// NewDispatcher creates a new command dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]func(map[string]interface{}) (interface{}, error)),
	}
}

// Register registers a handler for a command.
func (d *Dispatcher) Register(cmd string, handler func(map[string]interface{}) (interface{}, error)) {
	d.handlers[cmd] = handler
}

// Handle implements Handler.
func (d *Dispatcher) Handle(cmd string, args map[string]interface{}) (interface{}, error) {
	handler, ok := d.handlers[cmd]
	if !ok {
		return nil, errors.UnknownCommand(cmd)
	}
	return handler(args)
}

// RunDispatcher creates a dispatcher, registers handlers, and runs the IPC loop.
func RunDispatcher(register func(*Dispatcher)) error {
	d := NewDispatcher()
	register(d)
	return Run(d)
}
