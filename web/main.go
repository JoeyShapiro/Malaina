//go:build js && wasm

// TODO where is the idiomatic place to put main for wasm builds

package main

import (
	"bytes"
	"fmt"
	"malaina/internal"
	"syscall/js"
)

func main() {
	// Register a function callable from JavaScript
	js.Global().Set("CreateGraph", js.FuncOf(func(this js.Value, args []js.Value) any {
		handler := js.FuncOf(func(this js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			reject := promiseArgs[1]

			// Spawn goroutine - your existing code works here!
			go func() {
				// create buffer for CreateGraph to write to
				var buf []byte
				wr := bytes.NewBuffer(buf)

				err := internal.CreateGraph(wr, args[0].Int(), "", "", func(id, seen, queue int, err error) {
					if err != nil {
						if err.Error() == "Too Many Requests." {
							js.Global().Get("console").Call("warn", "Too Many Requests. Waiting for timeout...")
						}

						if err.Error() == "Waiting for timeout" {
							js.Global().Get("console").Call("warn", "Waiting for timeout...")
						}
					} else {
						js.Global().Get("console").Call("log", fmt.Sprintf("Querying: %d (%d / %d)", id, seen, seen+queue))
					}
				})
				if err != nil {
					reject.Invoke(js.ValueOf(err.Error()))
				} else {
					resolve.Invoke(js.ValueOf(wr.String()))
				}

				js.Global().Get("console").Call("log", "Graph created successfully")
			}()

			return nil
		})

		return js.Global().Get("Promise").New(handler)
	}))
	// js.Global().Get("console").Call("log", "WASM Go Initialized")

	// Keep the program running
	<-make(chan bool)
}
