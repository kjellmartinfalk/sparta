package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var Version = "dev"

type Config struct {
	Values     map[string]interface{} `yaml:"values"`
	Secrets    map[string]string      `yaml:"secrets"`
	ConfigName string
}

const (
	awsParameterStorePrefix = "aws_parameter_store:"
	awsSecretManagerPrefix  = "aws_secret_manager:"
)

func main() {
	var (
		templatePath string
		configFiles  []string
		outputDir    string
		showVersion  bool
	)

	rootCmd := &cobra.Command{
		Use:   "sparta",
		Short: "A template processor secrets integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Printf("sparta version %s\n", Version)
				return nil
			}

			if templatePath == "" {
				return fmt.Errorf("template flag is required")
			}
			return processTemplate(templatePath, configFiles, outputDir)
		},
	}

	rootCmd.Flags().StringVarP(&templatePath, "template", "t", "", "Path to template file or directory")
	rootCmd.Flags().StringArrayVarP(&configFiles, "config", "c", []string{}, "Path to config files")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (if not specified, outputs to stdout)")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func processTemplate(templatePath string, configFiles []string, outputDir string) error {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("error loading AWS config: %w", err)
	}

	for _, configFile := range configFiles {
		templateConfig, err := loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("error loading config %s: %w", configFile, err)
		}

		if err := loadSecrets(templateConfig, awsCfg); err != nil {
			return fmt.Errorf("error loading secrets for config %s: %w", configFile, err)
		}

		if err := processTemplates(templatePath, templateConfig, outputDir); err != nil {
			return fmt.Errorf("error processing templates with config %s: %w", configFile, err)
		}
	}

	return nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Values == nil {
		config.Values = make(map[string]interface{})
	}
	if config.Secrets == nil {
		config.Secrets = make(map[string]string)
	}

	config.ConfigName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &config, nil
}

func loadSecrets(config *Config, awsCfg aws.Config) error {
	ssmClient := ssm.NewFromConfig(awsCfg)
	secretsClient := secretsmanager.NewFromConfig(awsCfg)

	for name, path := range config.Secrets {
		switch {
		case strings.HasPrefix(path, awsParameterStorePrefix):
			paramPath := strings.TrimPrefix(path, awsParameterStorePrefix)
			param, err := ssmClient.GetParameter(context.Background(), &ssm.GetParameterInput{
				Name:           aws.String(paramPath),
				WithDecryption: aws.Bool(true),
			})
			if err != nil {
				return fmt.Errorf("error loading parameter %s: %w", paramPath, err)
			}
			config.Values[name] = *param.Parameter.Value

		case strings.HasPrefix(path, awsSecretManagerPrefix):
			secretID := strings.TrimPrefix(path, awsSecretManagerPrefix)
			secret, err := secretsClient.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(secretID),
			})
			if err != nil {
				return fmt.Errorf("error loading secret %s: %w", secretID, err)
			}
			config.Values[name] = *secret.SecretString

		default:
			return fmt.Errorf("unknown secret provider for %s", path)
		}
	}

	return nil
}

func extractJSONField(jsonStr string, path string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("field %s not found", part)
			}
		case []interface{}:
			return nil, fmt.Errorf("array indexing not supported yet")
		default:
			return nil, fmt.Errorf("invalid path: %s", part)
		}
	}

	return current, nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"jsonField": func(jsonStr string, path string) interface{} {
			val, err := extractJSONField(jsonStr, path)
			if err != nil {
				panic(fmt.Sprintf("Error extracting JSON field: %v", err))
			}
			return val
		},
		"b64enc": func(v string) string {
			return base64.StdEncoding.EncodeToString([]byte(v))
		},
		"b64dec": func(v string) string {
			data, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				panic(fmt.Sprintf("Error decoding base64: %v", err))
			}
			return string(data)
		},
	}
}

func processTemplates(path string, config *Config, outputDir string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
				return processTemplateFile(path, config, outputDir)
			}
			return nil
		})
	}

	return processTemplateFile(path, config, outputDir)
}

func processTemplateFile(path string, config *Config, outputDir string) error {
	tmplData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tmpl, err := template.New(filepath.Base(path)).
		Funcs(templateFuncs()).
		Parse(string(tmplData))
	if err != nil {
		return err
	}

	var output io.Writer
	if outputDir == "" {
		output = os.Stdout
	} else {
		outDir := filepath.Join(outputDir, config.ConfigName)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return err
		}

		outPath := filepath.Join(outDir, filepath.Base(path))
		file, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file

		fmt.Fprintf(os.Stderr, "Processing template: %s -> %s\n", path, outPath)
	}

	return tmpl.Execute(output, config.Values)
}
