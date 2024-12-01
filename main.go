package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Values     map[string]interface{} `yaml:"values"`
	SSMParams  map[string]string      `yaml:"ssm_params"`
	ConfigName string
}

func main() {
	var (
		templatePath string
		configFiles  []string
		outputDir    string
	)

	rootCmd := &cobra.Command{
		Use:   "tmpl",
		Short: "A template processor with AWS SSM integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return processTemplate(templatePath, configFiles, outputDir)
		},
	}

	rootCmd.Flags().StringVarP(&templatePath, "template", "t", "", "Path to template file or directory")
	rootCmd.Flags().StringArrayVarP(&configFiles, "config", "c", []string{}, "Path to config files")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (if not specified, outputs to stdout)")

	rootCmd.MarkFlagRequired("template")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func processTemplate(templatePath string, configFiles []string, outputDir string) error {
	for _, configFile := range configFiles {
		config, err := loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("error loading config %s: %w", configFile, err)
		}

		if err := loadSSMParams(config); err != nil {
			return fmt.Errorf("error loading SSM parameters for config %s: %w", configFile, err)
		}

		if err := processTemplates(templatePath, config, outputDir); err != nil {
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

	// Initialize maps if they're nil
	if config.Values == nil {
		config.Values = make(map[string]interface{})
	}
	if config.SSMParams == nil {
		config.SSMParams = make(map[string]string)
	}

	config.ConfigName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &config, nil
}

func loadSSMParams(templateConfig *Config) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	ssmClient := ssm.NewFromConfig(cfg)

	for name, path := range templateConfig.SSMParams {
		param, err := ssmClient.GetParameter(context.Background(), &ssm.GetParameterInput{
			Name:           aws.String(path),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return fmt.Errorf("error loading SSM parameter %s: %w", path, err)
		}

		templateConfig.Values[name] = *param.Parameter.Value
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
		"jsonField": func(jsonStr string, path string) (interface{}, error) {
			val, err := extractJSONField(jsonStr, path)
			if err != nil {
				return nil, err
			}
			return val, nil
		},
		"mustJsonField": func(jsonStr string, path string) interface{} {
			val, err := extractJSONField(jsonStr, path)
			if err != nil {
				panic(fmt.Sprintf("Error extracting JSON field: %v", err))
			}
			return val
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
