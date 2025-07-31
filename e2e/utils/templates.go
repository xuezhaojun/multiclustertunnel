package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// TemplateRenderer handles rendering of Kubernetes manifest templates
type TemplateRenderer struct {
	templateDir string
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(templateDir string) *TemplateRenderer {
	return &TemplateRenderer{
		templateDir: templateDir,
	}
}

// RenderTemplate renders a template file with the given parameters
func (tr *TemplateRenderer) RenderTemplate(templateFile string, params interface{}) (client.Object, error) {
	templatePath := filepath.Join(tr.templateDir, templateFile)

	// Create template with helper functions
	tmpl := template.New(filepath.Base(templateFile)).Funcs(template.FuncMap{
		"b64enc": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"b64dec": func(s string) (string, error) {
			data, err := base64.StdEncoding.DecodeString(s)
			return string(data), err
		},
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"ne": func(a, b interface{}) bool {
			return a != b
		},
		"quote": func(s interface{}) string {
			return fmt.Sprintf("%q", s)
		},
	})

	// Parse the template file
	tmpl, err := tmpl.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", templateFile, err)
	}

	// Execute the template with parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("failed to execute template %s: %w", templateFile, err)
	}

	// Decode the rendered YAML into a Kubernetes object
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}

	if _, _, err := decoder.Decode(buf.Bytes(), nil, obj); err != nil {
		return nil, fmt.Errorf("failed to decode rendered template %s: %w", templateFile, err)
	}

	return obj, nil
}

// ApplyTemplate renders and applies a template to the cluster
func (tr *TemplateRenderer) ApplyTemplate(ctx context.Context, cfg *envconf.Config, templateFile string, params interface{}) error {
	obj, err := tr.RenderTemplate(templateFile, params)
	if err != nil {
		return err
	}

	// Apply the object to the cluster
	if err := cfg.Client().Resources().Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to apply template %s: %w", templateFile, err)
	}

	return nil
}

// RenderTemplateToString renders a template to a string (useful for debugging)
func (tr *TemplateRenderer) RenderTemplateToString(templateFile string, params interface{}) (string, error) {
	templatePath := filepath.Join(tr.templateDir, templateFile)

	// Create template with helper functions
	tmpl := template.New(filepath.Base(templateFile)).Funcs(template.FuncMap{
		"b64enc": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"b64dec": func(s string) (string, error) {
			data, err := base64.StdEncoding.DecodeString(s)
			return string(data), err
		},
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"ne": func(a, b interface{}) bool {
			return a != b
		},
		"quote": func(s interface{}) string {
			return fmt.Sprintf("%q", s)
		},
	})

	tmpl, err := tmpl.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateFile, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateFile, err)
	}

	return buf.String(), nil
}

// Global template renderer instance (will be initialized in main_test.go)
var GlobalRenderer *TemplateRenderer

// Helper functions for use in test files

// RenderTemplate renders a template using the global renderer
func RenderTemplate(templateFile string, params interface{}) (client.Object, error) {
	if GlobalRenderer == nil {
		return nil, fmt.Errorf("global template renderer not initialized")
	}
	return GlobalRenderer.RenderTemplate(templateFile, params)
}

// ApplyTemplate applies a template using the global renderer
func ApplyTemplate(ctx context.Context, cfg *envconf.Config, templateFile string, params interface{}) error {
	if GlobalRenderer == nil {
		return fmt.Errorf("global template renderer not initialized")
	}
	return GlobalRenderer.ApplyTemplate(ctx, cfg, templateFile, params)
}

// RenderTemplateToString renders a template to string using the global renderer
func RenderTemplateToString(templateFile string, params interface{}) (string, error) {
	if GlobalRenderer == nil {
		return "", fmt.Errorf("global template renderer not initialized")
	}
	return GlobalRenderer.RenderTemplateToString(templateFile, params)
}
