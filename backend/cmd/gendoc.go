package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/docgen"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	routes  bool
	openapi bool
)

// gendocCmd represents the gendoc command
var gendocCmd = &cobra.Command{
	Use:   "gendoc",
	Short: "Generate project documentation",
	Long: `Generate documentation for the MOTO server API.

This command can generate:
- API routes markdown documentation
- OpenAPI specification (compatible with Swagger)

Use the appropriate flags to generate the desired documentation.`,
	Run: func(cmd *cobra.Command, args []string) {
		if routes {
			genRoutesDoc()
		}
		if openapi {
			genOpenAPIDoc()
		}
		if !routes && !openapi {
			// Default: generate both if no flags specified
			genRoutesDoc()
			genOpenAPIDoc()
		}
	},
}

func init() {
	RootCmd.AddCommand(gendocCmd)

	// Define flags for gendoc command
	gendocCmd.Flags().BoolVarP(&routes, "routes", "r", false, "create api routes markdown file")
	gendocCmd.Flags().BoolVarP(&openapi, "openapi", "o", false, "create or update OpenAPI specification")
}

func genRoutesDoc() {
	apiInstance, err := api.New(false)
	if err != nil {
		log.Fatalf("Failed to initialize API: %v", err)
	}

	// Use the Router field from the API instance, which is a chi.Router
	fmt.Print("Generating routes markdown file: ")
	md := docgen.MarkdownRoutesDoc(apiInstance.Router, docgen.MarkdownOpts{
		ProjectPath: "github.com/moto-nrw/project-phoenix",
		Intro:       "MOTO REST API for RFID-based system.",
	})
	if err := os.WriteFile("routes.md", []byte(md), 0644); err != nil {
		log.Println(err)
		return
	}
	fmt.Println("OK")
}

func genOpenAPIDoc() {
	fmt.Print("Generating OpenAPI specification: ")

	// Initialize API to get the router
	apiInstance, err := api.New(false)
	if err != nil {
		log.Fatalf("Failed to initialize API: %v", err)
	}

	// Ensure docs directory exists
	docsDir := "docs"
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		if err := os.Mkdir(docsDir, 0755); err != nil {
			log.Fatalf("Failed to create docs directory: %v", err)
		}
	}

	// Define the OpenAPI specification
	openAPIPath := filepath.Join(docsDir, "openapi.yaml")

	// Create base specification
	spec := createBaseOpenAPISpecFromRouter(apiInstance.Router)

	// Convert to YAML
	data, err := yaml.Marshal(spec)
	if err != nil {
		log.Fatalf("Failed to marshal OpenAPI spec: %v", err)
	}

	// Write to file
	if err := os.WriteFile(openAPIPath, data, 0644); err != nil {
		log.Fatalf("Failed to write OpenAPI spec to file: %v", err)
	}

	// Run swagger CLI to validate the spec if swag is installed
	if _, err := exec.LookPath("swag"); err == nil {
		cmd := exec.Command("swag", "fmt", "--dir", ".")
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: Failed to format with swagger: %v", err)
		}
	} else {
		fmt.Println("Swag CLI not found. Install with: go install github.com/swaggo/swag/cmd/swag@latest")
	}

	fmt.Println("OK - OpenAPI specification generated/updated at", openAPIPath)
}

// createBaseOpenAPISpecFromRouter creates an OpenAPI specification from the chi router
func createBaseOpenAPISpecFromRouter(router chi.Router) map[string]any {
	spec := createOpenAPIBaseStructure()
	md := docgen.MarkdownRoutesDoc(router, docgen.MarkdownOpts{})
	paths := spec["paths"].(map[string]any)

	parseRoutesFromMarkdown(md, paths)
	mergeSettingsSchemas(spec)

	return spec
}

// createOpenAPIBaseStructure creates the base OpenAPI specification structure
func createOpenAPIBaseStructure() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "MOTO API",
			"description": "API for the MOTO school management system",
			"version":     "1.0.0",
			"contact": map[string]any{
				"name": "MOTO Support",
			},
		},
		"servers": []map[string]any{
			{
				"url":         "/api",
				"description": "API Base URL",
			},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKeyAuth": map[string]any{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Authorization",
					"description": "API key for device authentication. Provide the API key as a Bearer token.",
				},
			},
			"schemas": map[string]any{},
		},
		"paths": map[string]any{},
	}
}

// parseRoutesFromMarkdown parses routes from markdown documentation
func parseRoutesFromMarkdown(md string, paths map[string]any) {
	lines := strings.Split(md, "\n")
	var currentRoute string

	for _, line := range lines {
		if route := extractRoutePattern(line); route != "" {
			currentRoute = route
			if paths[currentRoute] == nil {
				paths[currentRoute] = map[string]any{}
			}
		} else {
			tryAddHTTPMethod(line, paths, currentRoute)
		}
	}
}

// extractRoutePattern extracts route pattern from a markdown line
func extractRoutePattern(line string) string {
	if !strings.Contains(line, "`") || !strings.Contains(line, "<summary>") {
		return ""
	}

	routeStartIdx := strings.Index(line, "`") + 1
	routeEndIdx := strings.LastIndex(line, "`")

	if routeStartIdx <= 0 || routeEndIdx <= routeStartIdx {
		return ""
	}

	route := line[routeStartIdx:routeEndIdx]
	if route == "" || route == "*" {
		return ""
	}

	return route
}

// tryAddHTTPMethod tries to add HTTP method if the line contains a method marker
func tryAddHTTPMethod(line string, paths map[string]any, currentRoute string) {
	if currentRoute == "" {
		return
	}

	methods := map[string]string{
		"_GET_":    "GET",
		"_POST_":   "POST",
		"_PUT_":    "PUT",
		"_DELETE_": "DELETE",
		"_PATCH_":  "PATCH",
	}

	for marker, method := range methods {
		if strings.Contains(line, marker) {
			addMethod(paths, currentRoute, method)
			return
		}
	}
}

// mergeSettingsSchemas merges settings schemas into the OpenAPI spec
func mergeSettingsSchemas(spec map[string]any) {
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	settingSchemas := getSettingsSchemas()

	for name, schema := range settingSchemas {
		schemas[name] = schema
	}
}

// extractPathParams extracts URL parameters from a path pattern like /users/{id}
func extractPathParams(pattern string) []string {
	var params []string

	// Chi uses {paramName} for URL parameters
	// Regex to match {paramName} in the URL pattern
	r := regexp.MustCompile(`\{([^/]+)\}`)
	matches := r.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}

	return params
}

// getTagsFromPath returns API tags based on the URL path
func getTagsFromPath(path string) []string {
	parts := strings.Split(path, "/")

	// Find the first meaningful part of the path for tag
	for _, part := range parts {
		if part != "" && part != "api" {
			// Capitalize first letter of tag
			if len(part) > 0 {
				part = strings.ToUpper(part[:1]) + part[1:]
			}
			return []string{part}
		}
	}

	return []string{"API"}
}

// addMethod adds a method to a route in the paths map
func addMethod(paths map[string]any, route string, method string) {
	// Get or create methods map for this path
	pathInfo := paths[route].(map[string]any)

	// Convert to lowercase for OpenAPI
	methodLower := strings.ToLower(method)

	// Skip if method already exists
	if pathInfo[methodLower] != nil {
		return
	}

	// Create a basic operation for this method
	pathInfo[methodLower] = map[string]any{
		"summary":     fmt.Sprintf("%s %s", method, route),
		"description": "Generated from routes",
		"tags":        getTagsFromPath(route),
		"security": []map[string][]string{
			{"bearerAuth": {}},
		},
		"responses": map[string]any{
			"200": map[string]any{
				"description": "Successful operation",
			},
			"400": map[string]any{
				"description": "Bad request",
			},
			"401": map[string]any{
				"description": "Unauthorized",
			},
			"404": map[string]any{
				"description": "Not found",
			},
			"500": map[string]any{
				"description": "Internal server error",
			},
		},
	}

	// Add path parameters if any are in the route pattern
	pathParams := extractPathParams(route)
	if len(pathParams) > 0 {
		operation := pathInfo[methodLower].(map[string]any)
		parameters := []map[string]any{}

		for _, param := range pathParams {
			parameters = append(parameters, map[string]any{
				"name":        param,
				"in":          "path",
				"required":    true,
				"description": fmt.Sprintf("%s parameter", param),
				"schema": map[string]any{
					"type": "string", // Default to string type, can be refined manually later
				},
			})
		}

		operation["parameters"] = parameters
	}
}

// getSettingsSchemas returns a map of schema definitions for settings-related models
func getSettingsSchemas() map[string]any {
	return map[string]any{
		"Setting": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"format":      "int64",
					"description": "Unique identifier for the setting",
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Unique key identifying the setting",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Value of the setting",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Category the setting belongs to",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Description of the setting",
				},
				"requires_restart": map[string]any{
					"type":        "boolean",
					"description": "Indicates if the system needs to be restarted for the setting to take effect",
				},
				"requires_db_reset": map[string]any{
					"type":        "boolean",
					"description": "Indicates if the database needs to be reset for the setting to take effect",
				},
				"created_at": map[string]any{
					"type":        "string",
					"format":      "date-time",
					"description": "Timestamp when the setting was created",
				},
				"modified_at": map[string]any{
					"type":        "string",
					"format":      "date-time",
					"description": "Timestamp when the setting was last modified",
				},
			},
			"required": []string{"id", "key", "value", "category"},
		},
		"SettingRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Unique key identifying the setting",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Value of the setting",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Category the setting belongs to",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Description of the setting",
				},
				"requires_restart": map[string]any{
					"type":        "boolean",
					"description": "Indicates if the system needs to be restarted for the setting to take effect",
				},
				"requires_db_reset": map[string]any{
					"type":        "boolean",
					"description": "Indicates if the database needs to be reset for the setting to take effect",
				},
			},
			"required": []string{"key", "value", "category"},
		},
	}
}
