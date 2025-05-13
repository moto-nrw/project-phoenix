package cmd

import (
	"fmt"
	"github.com/moto-nrw/project-phoenix/api"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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
func createBaseOpenAPISpecFromRouter(router chi.Router) map[string]interface{} {
	// Define the base OpenAPI specification
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "MOTO API",
			"description": "API for the MOTO school management system",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "MOTO Support",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         "/api",
				"description": "API Base URL",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKeyAuth": map[string]interface{}{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Authorization",
					"description": "API key for device authentication. Provide the API key as a Bearer token.",
				},
			},
			"schemas": map[string]interface{}{},
		},
		"paths": map[string]interface{}{},
	}

	// Extract path information from the router using docgen's MarkdownRoutesDoc
	md := docgen.MarkdownRoutesDoc(router, docgen.MarkdownOpts{})

	// Parse the routes document to extract paths and methods
	paths := spec["paths"].(map[string]interface{})

	// Parse the routes from the markdown document
	// Since we can't directly walk the router, we'll extract routes from the markdown

	// Get all routes from our markdown doc
	lines := strings.Split(md, "\n")
	var currentRoute string

	for _, line := range lines {
		// Look for route patterns in the markdown using the summary tag format
		if strings.Contains(line, "`") && strings.Contains(line, "<summary>") {
			// Extract the route pattern
			routeStartIdx := strings.Index(line, "`") + 1
			routeEndIdx := strings.LastIndex(line, "`")
			if routeStartIdx > 0 && routeEndIdx > routeStartIdx {
				currentRoute = line[routeStartIdx:routeEndIdx]

				// Skip empty routes and middleware-only routes
				if currentRoute == "" || currentRoute == "*" {
					continue
				}

				// Initialize path if it doesn't exist
				if paths[currentRoute] == nil {
					paths[currentRoute] = map[string]interface{}{}
				}
			}
		} else if strings.Contains(line, "_GET_") && currentRoute != "" {
			addMethod(paths, currentRoute, "GET")
		} else if strings.Contains(line, "_POST_") && currentRoute != "" {
			addMethod(paths, currentRoute, "POST")
		} else if strings.Contains(line, "_PUT_") && currentRoute != "" {
			addMethod(paths, currentRoute, "PUT")
		} else if strings.Contains(line, "_DELETE_") && currentRoute != "" {
			addMethod(paths, currentRoute, "DELETE")
		} else if strings.Contains(line, "_PATCH_") && currentRoute != "" {
			addMethod(paths, currentRoute, "PATCH")
		}
	}

	// Add the settings schemas from the existing function
	schemas := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	settingSchemas := getSettingsSchemas()
	for name, schema := range settingSchemas {
		schemas[name] = schema
	}

	return spec
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
func addMethod(paths map[string]interface{}, route string, method string) {
	// Get or create methods map for this path
	pathInfo := paths[route].(map[string]interface{})

	// Convert to lowercase for OpenAPI
	methodLower := strings.ToLower(method)

	// Skip if method already exists
	if pathInfo[methodLower] != nil {
		return
	}

	// Create a basic operation for this method
	pathInfo[methodLower] = map[string]interface{}{
		"summary":     fmt.Sprintf("%s %s", method, route),
		"description": "Generated from routes",
		"tags":        getTagsFromPath(route),
		"security": []map[string][]string{
			{"bearerAuth": {}},
		},
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"description": "Successful operation",
			},
			"400": map[string]interface{}{
				"description": "Bad request",
			},
			"401": map[string]interface{}{
				"description": "Unauthorized",
			},
			"404": map[string]interface{}{
				"description": "Not found",
			},
			"500": map[string]interface{}{
				"description": "Internal server error",
			},
		},
	}

	// Add path parameters if any are in the route pattern
	pathParams := extractPathParams(route)
	if len(pathParams) > 0 {
		operation := pathInfo[methodLower].(map[string]interface{})
		parameters := []map[string]interface{}{}

		for _, param := range pathParams {
			parameters = append(parameters, map[string]interface{}{
				"name":        param,
				"in":          "path",
				"required":    true,
				"description": fmt.Sprintf("%s parameter", param),
				"schema": map[string]interface{}{
					"type": "string", // Default to string type, can be refined manually later
				},
			})
		}

		operation["parameters"] = parameters
	}
}

func createBaseOpenAPISpec(filePath string) {
	// Define the base OpenAPI specification
	baseSpec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "MOTO API",
			"description": "API for the MOTO school management system",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "MOTO Support",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         "/api",
				"description": "API Base URL",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKeyAuth": map[string]interface{}{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Authorization",
					"description": "API key for device authentication. Provide the API key as a Bearer token.",
				},
			},
			"schemas": map[string]interface{}{},
		},
		"paths": map[string]interface{}{},
	}

	// Convert to YAML
	data, err := yaml.Marshal(baseSpec)
	if err != nil {
		log.Fatalf("Failed to marshal OpenAPI spec: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to write OpenAPI spec to file: %v", err)
	}
}

func updateOpenAPISpec(filePath string) {
	// Read existing spec
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read existing OpenAPI spec: %v", err)
	}

	// Parse YAML to map - using interface{} for flexibility with different YAML parsers
	var rawSpec interface{}
	if err := yaml.Unmarshal(data, &rawSpec); err != nil {
		log.Fatalf("Failed to parse existing OpenAPI spec: %v", err)
	}

	// Convert the parsed data to a consistent map format
	spec := convertToStringMap(rawSpec)

	// Initialize paths if it doesn't exist
	if spec["paths"] == nil {
		spec["paths"] = map[string]interface{}{}
	}

	// Update paths and components with the latest API endpoints
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		paths = map[string]interface{}{}
		spec["paths"] = paths
	}

	// Get or create the components section
	if spec["components"] == nil {
		spec["components"] = map[string]interface{}{}
	}

	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		components = map[string]interface{}{}
		spec["components"] = components
	}

	// Get or create the schemas section
	if components["schemas"] == nil {
		components["schemas"] = map[string]interface{}{}
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		schemas = map[string]interface{}{}
		components["schemas"] = schemas
	}

	// Update settings API endpoints and schemas
	updateSettingsAPISpec(paths)

	// Add settings schemas to components
	settingSchemas := getSettingsSchemas()
	for name, schema := range settingSchemas {
		schemas[name] = schema
	}

	// Convert back to YAML
	updatedData, err := yaml.Marshal(spec)
	if err != nil {
		log.Fatalf("Failed to marshal updated OpenAPI spec: %v", err)
	}

	// Write back to file
	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		log.Fatalf("Failed to write updated OpenAPI spec to file: %v", err)
	}
}

// getSettingsSchemas returns a map of schema definitions for settings-related models
func getSettingsSchemas() map[string]interface{} {
	return map[string]interface{}{
		"Setting": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "integer",
					"format":      "int64",
					"description": "Unique identifier for the setting",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Unique key identifying the setting",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "Value of the setting",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Category the setting belongs to",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the setting",
				},
				"requires_restart": map[string]interface{}{
					"type":        "boolean",
					"description": "Indicates if the system needs to be restarted for the setting to take effect",
				},
				"requires_db_reset": map[string]interface{}{
					"type":        "boolean",
					"description": "Indicates if the database needs to be reset for the setting to take effect",
				},
				"created_at": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Timestamp when the setting was created",
				},
				"modified_at": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Timestamp when the setting was last modified",
				},
			},
			"required": []string{"id", "key", "value", "category"},
		},
		"SettingRequest": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Unique key identifying the setting",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "Value of the setting",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Category the setting belongs to",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the setting",
				},
				"requires_restart": map[string]interface{}{
					"type":        "boolean",
					"description": "Indicates if the system needs to be restarted for the setting to take effect",
				},
				"requires_db_reset": map[string]interface{}{
					"type":        "boolean",
					"description": "Indicates if the database needs to be reset for the setting to take effect",
				},
			},
			"required": []string{"key", "value", "category"},
		},
	}
}

func updateSettingsAPISpec(paths map[string]interface{}) {
	// Settings API endpoints
	settingsEndpoints := map[string]interface{}{
		"/settings": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List all settings",
				"description": "Returns a list of all system settings",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successfully retrieved settings list",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/Setting",
									},
								},
							},
						},
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
			"post": map[string]interface{}{
				"summary":     "Create a new setting",
				"description": "Creates a new system setting",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/SettingRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{
						"description": "Setting created successfully",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Setting",
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Invalid request",
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"409": map[string]interface{}{
						"description": "Conflict - Setting with this key already exists",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
		},
		"/settings/{id}": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get a setting by ID",
				"description": "Returns a single setting by its ID",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Setting ID",
						"schema": map[string]interface{}{
							"type": "integer",
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successfully retrieved setting",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Setting",
								},
							},
						},
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"404": map[string]interface{}{
						"description": "Setting not found",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
			"put": map[string]interface{}{
				"summary":     "Update a setting by ID",
				"description": "Updates an existing setting by its ID",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Setting ID",
						"schema": map[string]interface{}{
							"type": "integer",
						},
					},
				},
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/SettingRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Setting updated successfully",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Setting",
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Invalid request",
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"404": map[string]interface{}{
						"description": "Setting not found",
					},
					"409": map[string]interface{}{
						"description": "Conflict - Setting with this key already exists",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
			"delete": map[string]interface{}{
				"summary":     "Delete a setting by ID",
				"description": "Deletes a setting by its ID",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Setting ID",
						"schema": map[string]interface{}{
							"type": "integer",
						},
					},
				},
				"responses": map[string]interface{}{
					"204": map[string]interface{}{
						"description": "Setting deleted successfully",
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"404": map[string]interface{}{
						"description": "Setting not found",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
		},
		"/settings/key/{key}": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get a setting by key",
				"description": "Returns a single setting by its key",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "key",
						"in":          "path",
						"required":    true,
						"description": "Setting key",
						"schema": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successfully retrieved setting",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Setting",
								},
							},
						},
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"404": map[string]interface{}{
						"description": "Setting not found",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
			"patch": map[string]interface{}{
				"summary":     "Update a setting value by key",
				"description": "Updates the value of an existing setting by its key",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "key",
						"in":          "path",
						"required":    true,
						"description": "Setting key",
						"schema": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"value": map[string]interface{}{
										"type":        "string",
										"description": "New value for the setting",
									},
								},
								"required": []string{"value"},
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Setting value updated successfully",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Setting",
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Invalid request",
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"404": map[string]interface{}{
						"description": "Setting not found",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
		},
		"/settings/category/{category}": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get settings by category",
				"description": "Returns all settings in a specific category",
				"tags":        []string{"Settings"},
				"security": []map[string][]string{
					{"bearerAuth": {}},
				},
				"parameters": []map[string]interface{}{
					{
						"name":        "category",
						"in":          "path",
						"required":    true,
						"description": "Settings category",
						"schema": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successfully retrieved settings",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/Setting",
									},
								},
							},
						},
					},
					"401": map[string]interface{}{
						"description": "Unauthorized - Authentication required",
					},
					"500": map[string]interface{}{
						"description": "Internal server error",
					},
				},
			},
		},
	}

	// Update paths with settings endpoints
	for endpoint, operations := range settingsEndpoints {
		paths[endpoint] = operations
	}
}

// convertToStringMap converts YAML decoded maps to map[string]interface{} format
// which is needed for consistent operations and re-encoding
func convertToStringMap(i interface{}) map[string]interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, v := range x {
			switch k2 := k.(type) {
			case string:
				switch v2 := v.(type) {
				case map[interface{}]interface{}:
					m[k2] = convertToStringMap(v2)
				case []interface{}:
					m[k2] = convertToSlice(v2)
				default:
					m[k2] = v2
				}
			}
		}
		return m
	case map[string]interface{}:
		m := map[string]interface{}{}
		for k, v := range x {
			switch v2 := v.(type) {
			case map[interface{}]interface{}:
				m[k] = convertToStringMap(v2)
			case map[string]interface{}:
				m[k] = convertToStringMap(v2)
			case []interface{}:
				m[k] = convertToSlice(v2)
			default:
				m[k] = v2
			}
		}
		return m
	}

	// If it's not a map, return an empty one
	return map[string]interface{}{}
}

// convertToSlice processes each element of a slice, converting maps if needed
func convertToSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch v2 := v.(type) {
		case map[interface{}]interface{}:
			result[i] = convertToStringMap(v2)
		case map[string]interface{}:
			result[i] = convertToStringMap(v2)
		case []interface{}:
			result[i] = convertToSlice(v2)
		default:
			result[i] = v2
		}
	}
	return result
}
