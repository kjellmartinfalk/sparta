package secrets

type SecretProviderInitFn func(map[string]interface{}) (SecretFn, error)
type SecretFn func(key string) (any, error)
