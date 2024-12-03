package utilities

import "os"

func EnvVariable(v string) string {
	return os.Getenv(v)
}
