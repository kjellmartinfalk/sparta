package functions

import (
	"sync"
	"text/template"
)

var templateFunctionsMutex *sync.Mutex = &sync.Mutex{}
var TemplateFunctions template.FuncMap

func RegisterFunction(name string, fn any) {
	templateFunctionsMutex.Lock()
	defer templateFunctionsMutex.Unlock()

	TemplateFunctions[name] = fn
}
