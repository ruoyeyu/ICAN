// Package safelog records static event names and error types only. User input,
// provider responses, signed URLs, headers and credentials are never formatted.
package safelog

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func Printf(event string, args ...any) {
	kinds := []string{}
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			kinds = append(kinds, fmt.Sprintf("%T", err))
		}
	}
	log.Printf("event=%q errorTypes=%q", event, strings.Join(kinds, ","))
}
func Println(event string) { log.Print(event) }
func Fatal(err error) {
	log.Printf("fatal startup/serve error type=%T; check documented configuration and service health", err)
	os.Exit(1)
}
