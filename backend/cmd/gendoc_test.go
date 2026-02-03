package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestGendocCmd_Metadata(t *testing.T) {
	assert.Equal(t, "gendoc", gendocCmd.Use)
	assert.Contains(t, gendocCmd.Short, "Generate")
	assert.Contains(t, gendocCmd.Long, "OpenAPI")
	assert.NotNil(t, gendocCmd.Run)
}

func TestGendocCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "gendoc" {
			found = true
			break
		}
	}
	assert.True(t, found, "gendocCmd should be registered on RootCmd")
}

func TestGendocCmd_Flags(t *testing.T) {
	f := gendocCmd.Flags()
	assert.NotNil(t, f.Lookup("routes"))
	assert.NotNil(t, f.Lookup("openapi"))
}

func TestGendocCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	gendocCmd.SetOut(buf)
	gendocCmd.SetErr(buf)

	err := gendocCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "gendoc")
	assert.Contains(t, output, "--routes")
	assert.Contains(t, output, "--openapi")
}

// =============================================================================
// createOpenAPIBaseStructure Tests
// =============================================================================

func TestCreateOpenAPIBaseStructure(t *testing.T) {
	spec := createOpenAPIBaseStructure()

	assert.Equal(t, "3.0.3", spec["openapi"])

	info, ok := spec["info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "MOTO API", info["title"])
	assert.Equal(t, "1.0.0", info["version"])
	assert.Contains(t, info["description"].(string), "MOTO")

	contact, ok := info["contact"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "MOTO Support", contact["name"])

	servers, ok := spec["servers"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, servers, 1)
	assert.Equal(t, "/api", servers[0]["url"])

	components, ok := spec["components"].(map[string]interface{})
	require.True(t, ok)

	securitySchemes, ok := components["securitySchemes"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, securitySchemes, "bearerAuth")
	assert.Contains(t, securitySchemes, "apiKeyAuth")

	schemas, ok := components["schemas"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, schemas)

	paths, ok := spec["paths"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, paths)
}

// =============================================================================
// extractRoutePattern Tests
// =============================================================================

func TestExtractRoutePattern_WithBackticksAndSummary(t *testing.T) {
	line := "`/api/students` <summary>"
	result := extractRoutePattern(line)
	assert.Equal(t, "/api/students", result)
}

func TestExtractRoutePattern_NoBackticks(t *testing.T) {
	line := "/api/students <summary>"
	result := extractRoutePattern(line)
	assert.Equal(t, "", result)
}

func TestExtractRoutePattern_NoSummaryTag(t *testing.T) {
	line := "`/api/students`"
	result := extractRoutePattern(line)
	assert.Equal(t, "", result)
}

func TestExtractRoutePattern_EmptyLine(t *testing.T) {
	result := extractRoutePattern("")
	assert.Equal(t, "", result)
}

func TestExtractRoutePattern_WildcardRoute(t *testing.T) {
	line := "`*` <summary>"
	result := extractRoutePattern(line)
	assert.Equal(t, "", result)
}

func TestExtractRoutePattern_EmptyBackticks(t *testing.T) {
	line := "`` <summary>"
	result := extractRoutePattern(line)
	assert.Equal(t, "", result)
}

func TestExtractRoutePattern_SingleBacktick(t *testing.T) {
	line := "` <summary>"
	result := extractRoutePattern(line)
	assert.Equal(t, "", result)
}

// =============================================================================
// extractPathParams Tests
// =============================================================================

func TestExtractPathParams_SingleParam(t *testing.T) {
	params := extractPathParams("/users/{id}")
	assert.Equal(t, []string{"id"}, params)
}

func TestExtractPathParams_MultipleParams(t *testing.T) {
	params := extractPathParams("/users/{userId}/posts/{postId}")
	assert.Equal(t, []string{"userId", "postId"}, params)
}

func TestExtractPathParams_NoParams(t *testing.T) {
	params := extractPathParams("/api/students")
	assert.Nil(t, params)
}

func TestExtractPathParams_EmptyPath(t *testing.T) {
	params := extractPathParams("")
	assert.Nil(t, params)
}

func TestExtractPathParams_ComplexPath(t *testing.T) {
	params := extractPathParams("/api/{version}/groups/{groupId}/students/{studentId}")
	assert.Equal(t, []string{"version", "groupId", "studentId"}, params)
}

// =============================================================================
// getTagsFromPath Tests
// =============================================================================

func TestGetTagsFromPath_SimpleAPI(t *testing.T) {
	tags := getTagsFromPath("/api/students")
	assert.Equal(t, []string{"Students"}, tags)
}

func TestGetTagsFromPath_NestedPath(t *testing.T) {
	tags := getTagsFromPath("/api/groups/123/students")
	assert.Equal(t, []string{"Groups"}, tags)
}

func TestGetTagsFromPath_IoTPath(t *testing.T) {
	tags := getTagsFromPath("/api/iot/devices")
	assert.Equal(t, []string{"Iot"}, tags)
}

func TestGetTagsFromPath_RootPath(t *testing.T) {
	tags := getTagsFromPath("/")
	assert.Equal(t, []string{"API"}, tags)
}

func TestGetTagsFromPath_EmptyPath(t *testing.T) {
	tags := getTagsFromPath("")
	assert.Equal(t, []string{"API"}, tags)
}

func TestGetTagsFromPath_APIOnly(t *testing.T) {
	// /api with nothing after should fall through to "API" default
	tags := getTagsFromPath("/api")
	assert.Equal(t, []string{"API"}, tags)
}

func TestGetTagsFromPath_NonAPIPrefix(t *testing.T) {
	tags := getTagsFromPath("/health")
	assert.Equal(t, []string{"Health"}, tags)
}

// =============================================================================
// getSettingsSchemas Tests
// =============================================================================

func TestGetSettingsSchemas(t *testing.T) {
	schemas := getSettingsSchemas()

	assert.Contains(t, schemas, "Setting")
	assert.Contains(t, schemas, "SettingRequest")

	setting, ok := schemas["Setting"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", setting["type"])

	props, ok := setting["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "id")
	assert.Contains(t, props, "key")
	assert.Contains(t, props, "value")
	assert.Contains(t, props, "category")
	assert.Contains(t, props, "description")
	assert.Contains(t, props, "requires_restart")
	assert.Contains(t, props, "requires_db_reset")
	assert.Contains(t, props, "created_at")
	assert.Contains(t, props, "modified_at")

	required, ok := setting["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "id")
	assert.Contains(t, required, "key")
	assert.Contains(t, required, "value")
	assert.Contains(t, required, "category")

	settingReq, ok := schemas["SettingRequest"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", settingReq["type"])

	reqProps, ok := settingReq["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, reqProps, "key")
	assert.Contains(t, reqProps, "value")
	assert.Contains(t, reqProps, "category")

	reqRequired, ok := settingReq["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, reqRequired, "key")
	assert.Contains(t, reqRequired, "value")
	assert.Contains(t, reqRequired, "category")
}

// =============================================================================
// tryAddHTTPMethod Tests
// =============================================================================

func TestTryAddHTTPMethod_EmptyCurrentRoute(t *testing.T) {
	paths := map[string]interface{}{}
	tryAddHTTPMethod("_GET_ /some/route", paths, "")
	assert.Empty(t, paths)
}

func TestTryAddHTTPMethod_GET(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("_GET_ handler", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Contains(t, pathInfo, "get")
}

func TestTryAddHTTPMethod_POST(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("_POST_ handler", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Contains(t, pathInfo, "post")
}

func TestTryAddHTTPMethod_PUT(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("_PUT_ handler", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Contains(t, pathInfo, "put")
}

func TestTryAddHTTPMethod_DELETE(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("_DELETE_ handler", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Contains(t, pathInfo, "delete")
}

func TestTryAddHTTPMethod_PATCH(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("_PATCH_ handler", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Contains(t, pathInfo, "patch")
}

func TestTryAddHTTPMethod_NoMethodMarker(t *testing.T) {
	paths := map[string]interface{}{
		"/api/test": map[string]interface{}{},
	}
	tryAddHTTPMethod("some random line", paths, "/api/test")

	pathInfo := paths["/api/test"].(map[string]interface{})
	assert.Empty(t, pathInfo)
}

// =============================================================================
// addMethod Tests
// =============================================================================

func TestAddMethod_CreatesOperation(t *testing.T) {
	paths := map[string]interface{}{
		"/api/students": map[string]interface{}{},
	}

	addMethod(paths, "/api/students", "GET")

	pathInfo := paths["/api/students"].(map[string]interface{})
	op, ok := pathInfo["get"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, op["summary"], "GET /api/students")
	assert.Equal(t, []string{"Students"}, op["tags"])
}

func TestAddMethod_SkipsDuplicate(t *testing.T) {
	paths := map[string]interface{}{
		"/api/students": map[string]interface{}{},
	}

	addMethod(paths, "/api/students", "GET")
	addMethod(paths, "/api/students", "GET") // should not panic or overwrite

	pathInfo := paths["/api/students"].(map[string]interface{})
	assert.Contains(t, pathInfo, "get")
}

func TestAddMethod_WithPathParams(t *testing.T) {
	paths := map[string]interface{}{
		"/api/students/{id}": map[string]interface{}{},
	}

	addMethod(paths, "/api/students/{id}", "GET")

	pathInfo := paths["/api/students/{id}"].(map[string]interface{})
	op, ok := pathInfo["get"].(map[string]interface{})
	require.True(t, ok)

	params, ok := op["parameters"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, params, 1)
	assert.Equal(t, "id", params[0]["name"])
	assert.Equal(t, "path", params[0]["in"])
	assert.Equal(t, true, params[0]["required"])
}

func TestAddMethod_WithMultiplePathParams(t *testing.T) {
	paths := map[string]interface{}{
		"/api/groups/{groupId}/students/{studentId}": map[string]interface{}{},
	}

	addMethod(paths, "/api/groups/{groupId}/students/{studentId}", "DELETE")

	pathInfo := paths["/api/groups/{groupId}/students/{studentId}"].(map[string]interface{})
	op, ok := pathInfo["delete"].(map[string]interface{})
	require.True(t, ok)

	params, ok := op["parameters"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, params, 2)
}

// =============================================================================
// mergeSettingsSchemas Tests
// =============================================================================

func TestMergeSettingsSchemas(t *testing.T) {
	spec := createOpenAPIBaseStructure()

	mergeSettingsSchemas(spec)

	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})

	assert.Contains(t, schemas, "Setting")
	assert.Contains(t, schemas, "SettingRequest")
}

// =============================================================================
// parseRoutesFromMarkdown Tests
// =============================================================================

func TestParseRoutesFromMarkdown_Empty(t *testing.T) {
	paths := map[string]interface{}{}
	parseRoutesFromMarkdown("", paths)
	assert.Empty(t, paths)
}

func TestParseRoutesFromMarkdown_WithRoutes(t *testing.T) {
	md := "`/api/students` <summary>\n_GET_ handler\n_POST_ handler\n"
	paths := map[string]interface{}{}
	parseRoutesFromMarkdown(md, paths)

	assert.Contains(t, paths, "/api/students")
	pathInfo := paths["/api/students"].(map[string]interface{})
	assert.Contains(t, pathInfo, "get")
	assert.Contains(t, pathInfo, "post")
}

func TestParseRoutesFromMarkdown_MultipleRoutes(t *testing.T) {
	md := "`/api/students` <summary>\n_GET_ handler\n`/api/groups` <summary>\n_POST_ handler\n"
	paths := map[string]interface{}{}
	parseRoutesFromMarkdown(md, paths)

	assert.Contains(t, paths, "/api/students")
	assert.Contains(t, paths, "/api/groups")
}
