package statestore

import "strings"

var readOnlyPrefixes = []string{
	"Find",
	"Get",
	"List",
	"Search",
	"Version",
	"Describe",
}

// readOnlyExceptions are read-only by prefix but still mutate mock state.
var mutatingMethods = map[string]struct{}{
	"FindInstancesByIds": {},
}

// IsMutatingMethod reports whether a gRPC full method name mutates mock state.
func IsMutatingMethod(fullMethod string) bool {
	name := methodBaseName(fullMethod)
	if _, ok := mutatingMethods[name]; ok {
		return true
	}
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func methodBaseName(fullMethod string) string {
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[i+1:]
	}
	return fullMethod
}
