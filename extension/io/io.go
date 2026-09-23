// Package io exports the stream shapes as traits. A user type can implement
// Reader or Writer instead of defining plain read/write methods; every stdlib
// API that accepts a stream takes either form.
package io

import "github.com/aisk/goblin/object"

func Execute() (object.Object, error) {
	return &object.Module{
		Name: "io",
		Members: map[string]object.Object{
			"Reader": object.ReaderTrait,
			"Writer": object.WriterTrait,
		},
	}, nil
}
