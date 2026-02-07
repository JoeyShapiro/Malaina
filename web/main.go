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
		var progressCb js.Value
		if len(args) > 1 && args[1].Type() == js.TypeFunction {
			progressCb = args[1]
		}

		handler := js.FuncOf(func(this js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			reject := promiseArgs[1]

			// Spawn goroutine - your existing code works here!
			go func() {
				// create buffer for CreateGraph to write to
				var buf []byte
				wr := bytes.NewBuffer(buf)

				err := internal.CreateGraph(wr, args[0].Int(), "", "", func(id, seen, queue int, err error) {
					if progressCb.Truthy() {
						payload := map[string]any{
							"id":    id,
							"seen":  seen,
							"queue": queue,
						}
						if err != nil {
							payload["error"] = err.Error()
						}
						progressCb.Invoke(js.ValueOf(payload))
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

	js.Global().Set("Search", js.FuncOf(func(this js.Value, args []js.Value) any {
		handler := js.FuncOf(func(this js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			reject := promiseArgs[1]

			go func() {
				media, err := internal.SearchAnime(args[0].String())
				if err != nil {
					reject.Invoke(js.ValueOf(err.Error()))
				} else {
					// cant get array to work. but id should be unique by nature
					payload := make(map[string]any)
					for _, m := range media {
						title := m.Title.English
						if title == "" {
							title = m.Title.Romaji
						}
						payload[fmt.Sprint(m.Id)] = title
					}

					resolve.Invoke(js.ValueOf(payload))
				}
			}()

			return nil
		})

		return js.Global().Get("Promise").New(handler)
	}))

	// Keep the program running
	<-make(chan bool)
}
