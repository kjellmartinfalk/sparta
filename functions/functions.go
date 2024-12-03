package functions

import (
	"sync"
	"text/template"

	secrets "github.com/kjellmartinfalk/sparta/functions/secret_providers"
	"github.com/kjellmartinfalk/sparta/functions/utilities"
)

var functionsMutex *sync.Mutex = &sync.Mutex{}
var TemplateFunctions template.FuncMap = template.FuncMap{
	"jsonField":     utilities.JsonField,
	"mustJsonField": utilities.MustJsonField,
	"b64enc":        utilities.Base64Encode,
	"b64dec":        utilities.Base64Decode,
}

var SecretProviders map[string]secrets.SecretProviderInitFn = map[string]secrets.SecretProviderInitFn{
	"aws_secret_manager": secrets.InitializeAwsSecretManager,
	"aws_ssm_parameters": secrets.InitializeAwsSsmParameters,
}
