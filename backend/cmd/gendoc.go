package cmd

import (
	"fmt"
	// "github.com/moto-nrw/project-phoenix/api" // Commented out temporarily until new API implementation
	"log"
	"os"
	"os/exec"
	"path/filepath"

	// "github.com/go-chi/docgen"
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
	// Temporarily commented out until new API implementation
	// api, err := api.New(false)
	// if err != nil {
	// 	log.Fatalf("Failed to initialize API: %v", err)
	// }
	//
	// fmt.Print("Generating routes markdown file: ")
	// md := docgen.MarkdownRoutesDoc(api, docgen.MarkdownOpts{
	// 	ProjectPath: "github.com/moto-nrw/project-phoenix",
	// 	Intro:       "MOTO REST API for RFID-based system.",
	// })
	// if err := os.WriteFile("routes.md", []byte(md), 0644); err != nil {
	// 	log.Println(err)
	// 	return
	// }
	// fmt.Println("OK")
	fmt.Println("Routes documentation generation is temporarily disabled during API refactoring")
}

func genOpenAPIDoc() {
	fmt.Print("Generating OpenAPI specification: ")

	// Ensure docs directory exists
	docsDir := "docs"
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		if err := os.Mkdir(docsDir, 0755); err != nil {
			log.Fatalf("Failed to create docs directory: %v", err)
		}
	}

	// Define the OpenAPI specification
	openAPIPath := filepath.Join(docsDir, "openapi.yaml")

	// Check if we need to update an existing spec or create a new one
	if _, err := os.Stat(openAPIPath); os.IsNotExist(err) {
		// Create a new specification file
		createBaseOpenAPISpec(openAPIPath)
	} else {
		// Update the existing specification
		updateOpenAPISpec(openAPIPath)
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
