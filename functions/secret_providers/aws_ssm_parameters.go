package secrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func InitializeAwsSsmParameters(c map[string]interface{}) (SecretFn, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := ssm.NewFromConfig(cfg)

	fn := func(key string) (any, error) {
		param, err := client.GetParameter(context.Background(), &ssm.GetParameterInput{
			Name:           aws.String(key),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return nil, fmt.Errorf("error loading parameter from parameter store, %s: %w", key, err)
		}

		return *param.Parameter.Value, nil
	}

	return fn, nil
}
