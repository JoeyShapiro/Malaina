//go:build js && wasm

// TODO where is the idiomatic place to put main for wasm builds

package main

import (
	"syscall/js"
)

func main() {
	// Register a function callable from JavaScript
	js.Global().Set("add", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return args[0].Int() + args[1].Int()
	}))
	js.Global().Get("console").Call("log", "WASM Go Initialized")

	// Keep the program running
	<-make(chan bool)
}
