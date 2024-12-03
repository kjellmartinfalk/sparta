package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/kjellmartinfalk/sparta/functions"
	"github.com/kjellmartinfalk/sparta/functions/secret_providers"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var Version string

type Config struct {
	Values          map[string]interface{}            `yaml:"values"`
	Secrets         map[string]string                 `yaml:"secrets"`
	SecretProviders map[string]map[string]interface{} `yaml:"secret_providers"`
	ConfigName      string
}

func main() {
	var (
		templatePath string
		configFiles  []string
		outputDir    string
		showVersion  bool
	)

	rootCmd := &cobra.Command{
		Use:   "sparta",
		Short: "a simple template processor",
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
	for _, configFile := range configFiles {
		templateConfig, err := loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("error loading config %s: %w", configFile, err)
		}

		if err := loadSecrets(templateConfig); err != nil {
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

	if config.SecretProviders == nil {
		config.SecretProviders = make(map[string]map[string]interface{})
	}

	if config.Secrets == nil {
		config.Secrets = make(map[string]string)
	}

	config.ConfigName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &config, nil
}

var (
	errMissingSecretProvider       = errors.New("secret identifier is to short, missing provider?")
	errMissingSecretProviderConfig = errors.New("missing provider config for key ..")
)

func loadSecrets(config *Config) error {
	initializedProviders := make(map[string]secrets.SecretFn)

	for name, path := range config.Secrets {
		secretKey := strings.Split(path, ":")

		if len(secretKey) < 2 {
			return errMissingSecretProvider
		}

		var providerIdentifier string
		var providerConfigIdentifier string
		var secretIdentifier string
		if len(secretIdentifier) == 3 {
			providerIdentifier = secretKey[0]
			providerConfigIdentifier = secretKey[1]
			secretIdentifier = secretKey[2]
		}

		providerIdentifier = secretKey[0]
		secretIdentifier = secretKey[1]

		provider, ok := initializedProviders[providerIdentifier+providerConfigIdentifier]
		if !ok {
			providerInitializer, ok := functions.SecretProviders[providerIdentifier]
			if !ok {
				return errors.New("missing secret provider " + providerIdentifier)
			}

			providerConfig := config.SecretProviders[providerConfigIdentifier]

			var err error
			provider, err = providerInitializer(providerConfig)
			if err != nil {
				return err
			}
		}

		secret, err := provider(secretIdentifier)
		if err != nil {
			return err
		}

		config.Values[name] = secret
	}

	return nil
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

	tmpl, err := template.New(filepath.Base(path)).Funcs(functions.TemplateFunctions).Parse(string(tmplData))
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
