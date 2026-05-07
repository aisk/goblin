package extension

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aisk/goblin/object"
)

// path → handler function
var httpHandlers = map[string]*object.Function{}

var httpModule = &object.Module{
	Members: map[string]object.Object{
		"handle_func": &object.Function{
			Name: "handle_func",
			Fn:   httpHandleFunc,
		},
		"listen_and_serve": &object.Function{
			Name: "listen_and_serve",
			Fn:   httpListenAndServe,
		},
	},
}

func httpHandleFunc(args object.CallArgs) (object.Object, error) {
	if len(args.Positional) != 2 {
		return nil, fmt.Errorf("http.handle_func() takes exactly 2 arguments: path and handler function")
	}
	pattern := args.Positional[0].String()
	fn, ok := args.Positional[1].(*object.Function)
	if !ok {
		return nil, fmt.Errorf("http.handle_func(): second argument must be a function, got %T", args.Positional[1])
	}
	httpHandlers[pattern] = fn
	return nil, nil
}

func httpListenAndServe(args object.CallArgs) (object.Object, error) {
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("http.listen_and_serve() takes exactly 1 argument: address")
	}
	addr := args.Positional[0].String()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fn, ok := httpHandlers[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		// Build request dict
		reqDict := object.NewDict()
		reqDict.Set(object.String("method"), object.String(r.Method))
		reqDict.Set(object.String("path"), object.String(r.URL.Path))
		reqDict.Set(object.String("query"), object.String(r.URL.RawQuery))

		headers := object.NewDict()
		for key, values := range r.Header {
			headers.Set(object.String(key), object.String(strings.Join(values, ", ")))
		}
		reqDict.Set(object.String("headers"), headers)

		result, err := fn.Fn(object.CallArgs{
			Positional: []object.Object{reqDict},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeResponse(w, result)
	})

	fmt.Printf("HTTP server listening on %s\n", addr)
	return nil, http.ListenAndServe(addr, mux)
}

func writeResponse(w http.ResponseWriter, result object.Object) {
	switch v := result.(type) {
	case object.String:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(v))
	case *object.Dict:
		status := http.StatusOK
		if s, ok := v.Get(object.String("status")); ok {
			if n, ok2 := s.(object.Integer); ok2 {
				status = int(n)
			}
		}

		if h, ok := v.Get(object.String("headers")); ok {
			if hDict, ok2 := h.(*object.Dict); ok2 {
				for _, entry := range hDict.Entries {
					w.Header().Set(entry.Key.String(), entry.Value.String())
				}
			}
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		if b, ok := v.Get(object.String("body")); ok {
			w.WriteHeader(status)
			fmt.Fprint(w, b.String())
		} else {
			w.WriteHeader(status)
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, result.String())
	}
}

// ExecuteHTTP returns the http module.
func ExecuteHTTP() (object.Object, error) {
	return httpModule, nil
}
