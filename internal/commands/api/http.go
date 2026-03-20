package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/api"
)

const (
	// stringArrayRegexPattern represents a pattern to find strings like: [item, item_two]
	stringArrayRegexPattern = `^\[\s*([[:lower:]_]+(\s*,\s*[[:lower:]_]+)*)?\s*\]$`
)

var strArrayRegex = regexp.MustCompile(stringArrayRegexPattern)

func httpRequest(ctx context.Context, client *api.Client, method, p string, params any, headers []string) (*http.Response, error) {
	var err error
	isGraphQL := p == "graphql"

	baseURL := client.Lab().BaseURL()
	baseURLStr := baseURL.String()
	if strings.Contains(p, "://") {
		baseURLStr = p
	} else if isGraphQL {
		baseURL.Path = strings.TrimSuffix(strings.TrimSuffix(baseURL.Path, "/"), "/api/v4") + "/api/graphql"
		baseURLStr = baseURL.String()
	} else {
		baseURLStr = baseURLStr + strings.TrimPrefix(p, "/")
	}

	var body io.Reader
	var bodyIsJSON bool
	switch pp := params.(type) {
	case map[string]any:
		if strings.EqualFold(method, http.MethodGet) || strings.EqualFold(method, http.MethodDelete) {
			baseURLStr, err = parseQuery(baseURLStr, pp)
			if err != nil {
				return nil, err
			}
		} else {
			for key, value := range pp {
				if vv, ok := value.([]byte); ok {
					pp[key] = string(vv)
				}

				if strValue, ok := value.(string); ok && strArrayRegex.MatchString(strValue) {
					pp[key] = parseStringArrayField(strValue)
				}
			}
			if isGraphQL {
				pp = groupGraphQLVariables(pp)
			}

			b, err := json.Marshal(pp)
			if err != nil {
				return nil, fmt.Errorf("error serializing parameters: %w", err)
			}
			body = bytes.NewBuffer(b)
			bodyIsJSON = true
		}
	case io.Reader:
		body = pp
	case nil:
		body = nil
	default:
		return nil, fmt.Errorf("unrecognized parameter type: %v", params)
	}

	baseURL, _ = url.Parse(baseURLStr)
	req, err := api.NewHTTPRequest(ctx, client, method, baseURL, body, headers, bodyIsJSON)
	if err != nil {
		return nil, err
	}
	return client.HTTPClient().Do(req)
}

func groupGraphQLVariables(params map[string]any) map[string]any {
	topLevel := make(map[string]any)
	variables := make(map[string]any)

	for key, val := range params {
		switch key {
		case "query", "operationName":
			topLevel[key] = val
		default:
			variables[key] = val
		}
	}

	if len(variables) > 0 {
		topLevel["variables"] = variables
	}
	return topLevel
}

func parseQuery(path string, params map[string]any) (string, error) {
	if len(params) == 0 {
		return path, nil
	}
	q := url.Values{}
	for key, value := range params {
		switch v := value.(type) {
		case string:
			q.Add(key, v)
		case []byte:
			q.Add(key, string(v))
		case nil:
			q.Add(key, "")
		case int:
			q.Add(key, fmt.Sprintf("%d", v))
		case bool:
			q.Add(key, fmt.Sprintf("%v", v))
		default:
			return "", fmt.Errorf("unknown type %v", v)
		}
	}

	sep := "?"
	if strings.ContainsRune(path, '?') {
		sep = "&"
	}
	return path + sep + q.Encode(), nil
}

// buildMultipartBody constructs a multipart/form-data request body from a slice
// of raw "key=value" or "key=@filepath" strings. File fields use the @filepath
// syntax; all other fields are written as plain text parts. The returned
// contentType string includes the boundary and must be set as the request's
// Content-Type header.
func buildMultipartBody(formFields []string, stdin io.ReadCloser) (*bytes.Buffer, string, error) {
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)

	for _, f := range formFields {
		key, value, err := parseField(f)
		if err != nil {
			return nil, "", err
		}

		if strings.HasPrefix(value, "@") {
			path := value[1:]
			var r io.Reader
			var filename string
			if path == "-" {
				r = stdin
				filename = "-"
			} else {
				fh, err := os.Open(path)
				if err != nil {
					return nil, "", err
				}
				defer fh.Close()
				r = fh
				filename = filepath.Base(path)
			}
			fw, err := w.CreateFormFile(key, filename)
			if err != nil {
				return nil, "", err
			}
			if _, err := io.Copy(fw, r); err != nil {
				return nil, "", err
			}
		} else {
			if err := w.WriteField(key, value); err != nil {
				return nil, "", err
			}
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return b, w.FormDataContentType(), nil
}

func parseStringArrayField(strValue string) []string {
	strValue = strings.TrimPrefix(strValue, "[")
	strValue = strings.TrimSuffix(strValue, "]")
	strArrayElements := strings.Split(strValue, ",")

	var strSlice []string
	for _, element := range strArrayElements {
		element = strings.TrimSpace(element)
		element = strings.Trim(element, `"`)
		strSlice = append(strSlice, element)
	}

	return strSlice
}
