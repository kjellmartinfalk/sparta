package secrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func InitializeAwsSecretManager(c map[string]interface{}) (SecretFn, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := secretsmanager.NewFromConfig(cfg)

	fn := func(key string) (any, error) {
		secret, err := client.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(key),
		})
		if err != nil {
			return nil, fmt.Errorf("error loading secret from secret manager, %s: %w", key, err)
		}

		return *secret.SecretString, nil
	}

	return fn, nil
}
