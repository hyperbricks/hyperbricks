package composite

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	"golang.org/x/net/publicsuffix"
)

var apiResponseCookieFields = map[string]struct{}{
	"name":      {},
	"value":     {},
	"path":      {},
	"domain":    {},
	"http_only": {},
	"secure":    {},
	"same_site": {},
	"max_age":   {},
	"expires":   {},
}

type apiResponseCookieSpec struct {
	cookie           http.Cookie
	valueTemplate    *template.Template
	valueFields      [][]string
	dynamicValue     bool
	allowsEmptyValue bool
}

// ValidateAPIResponseCookiesRaw validates the cookie-specific shape before a
// general-purpose decoder can coerce values. setcookie remains the legacy
// single-string shorthand. setcookies accepts legacy strings and structured
// cookie maps.
func ValidateAPIResponseCookiesRaw(raw map[string]interface{}) error {
	if raw == nil {
		return nil
	}

	if value, exists := raw["setcookie"]; exists {
		single, ok := value.(string)
		if !ok {
			return fmt.Errorf("setcookie must be a string, got %T", value)
		}
		if err := rejectAPIResponseCookieControlBytes(single); err != nil {
			return fmt.Errorf("setcookie: %w", err)
		}
		if strings.TrimSpace(single) != "" {
			if _, err := parseLegacyAPIResponseCookie(single); err != nil {
				return fmt.Errorf("setcookie: %w", err)
			}
		}
	}

	value, exists := raw["setcookies"]
	if !exists {
		return nil
	}

	var entries []interface{}
	switch typed := value.(type) {
	case []interface{}:
		entries = typed
	case []string:
		entries = make([]interface{}, len(typed))
		for index := range typed {
			entries[index] = typed[index]
		}
	default:
		return fmt.Errorf("setcookies must be a list of strings or cookie maps, got %T", value)
	}

	for index, entry := range entries {
		if rawCookie, ok := entry.(string); ok {
			if err := rejectAPIResponseCookieControlBytes(rawCookie); err != nil {
				return fmt.Errorf("setcookies[%d]: %w", index, err)
			}
			if strings.TrimSpace(rawCookie) == "" {
				continue
			}
		}
		if _, err := parseAPIResponseCookieEntry(entry); err != nil {
			return fmt.Errorf("setcookies[%d]: %w", index, err)
		}
	}
	return nil
}

// RenderAPIResponseCookies renders and validates response cookies without
// mutating an http.ResponseWriter. The returned header values are all-or-none:
// any invalid cookie discards the complete staged result.
func RenderAPIResponseCookies(single string, entries []interface{}, data interface{}, status int, values map[string]interface{}, incoming *http.Request) ([]string, error) {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, nil
	}

	specs := make([]apiResponseCookieSpec, 0, 1+len(entries))
	if err := rejectAPIResponseCookieControlBytes(single); err != nil {
		return nil, fmt.Errorf("setcookie: %w", err)
	}
	if strings.TrimSpace(single) != "" {
		spec, err := parseLegacyAPIResponseCookie(single)
		if err != nil {
			return nil, fmt.Errorf("setcookie: %w", err)
		}
		specs = append(specs, spec)
	}
	for index, entry := range entries {
		if rawCookie, ok := entry.(string); ok {
			if err := rejectAPIResponseCookieControlBytes(rawCookie); err != nil {
				return nil, fmt.Errorf("setcookies[%d]: %w", index, err)
			}
			if strings.TrimSpace(rawCookie) == "" {
				continue
			}
		}
		spec, err := parseAPIResponseCookieEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("setcookies[%d]: %w", index, err)
		}
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		return nil, nil
	}

	templateContext, err := apiResponseCookieTemplateContext(data, status, values)
	if err != nil {
		return nil, err
	}

	staged := make([]string, 0, len(specs))
	for index := range specs {
		spec := &specs[index]
		value, err := renderAPIResponseCookieValue(spec, templateContext)
		if err != nil {
			return nil, fmt.Errorf("response cookie %q: %w", spec.cookie.Name, err)
		}
		if value == "" {
			if spec.dynamicValue {
				return nil, fmt.Errorf("response cookie %q: dynamic value must resolve to a nonempty string", spec.cookie.Name)
			}
			if !spec.allowsEmptyValue {
				return nil, fmt.Errorf("response cookie %q: empty value requires an explicit Max-Age=0 deletion", spec.cookie.Name)
			}
		}

		cookie := spec.cookie
		cookie.Value = value
		if err := validateAPIResponseCookie(&cookie, incoming); err != nil {
			return nil, fmt.Errorf("response cookie %q: %w", cookie.Name, err)
		}
		header := cookie.String()
		if header == "" {
			return nil, fmt.Errorf("response cookie %q: failed to serialize cookie", cookie.Name)
		}
		staged = append(staged, header)
	}

	return staged, nil
}

func parseAPIResponseCookieEntry(entry interface{}) (apiResponseCookieSpec, error) {
	switch typed := entry.(type) {
	case string:
		return parseLegacyAPIResponseCookie(typed)
	case map[string]interface{}:
		return parseStructuredAPIResponseCookie(typed)
	default:
		return apiResponseCookieSpec{}, fmt.Errorf("cookie entry must be a string or map, got %T", entry)
	}
}

func parseLegacyAPIResponseCookie(raw string) (apiResponseCookieSpec, error) {
	if err := rejectAPIResponseCookieControlBytes(raw); err != nil {
		return apiResponseCookieSpec{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie string must not be blank")
	}

	pair := raw
	attributes := ""
	if separator := strings.IndexByte(raw, ';'); separator >= 0 {
		pair = raw[:separator]
		attributes = raw[separator+1:]
	}
	name, valueSource, found := strings.Cut(pair, "=")
	if !found {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie must start with name=value")
	}
	if name != strings.TrimSpace(name) {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie name must not contain surrounding whitespace")
	}
	if containsTemplateSyntax(name) {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie name must not contain template expressions")
	}
	if containsTemplateSyntax(attributes) {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie attributes must not contain template expressions")
	}
	if err := validateLegacyAPIResponseCookieAttributes(attributes); err != nil {
		return apiResponseCookieSpec{}, err
	}

	quoted := false
	if strings.HasPrefix(valueSource, `"`) || strings.HasSuffix(valueSource, `"`) {
		if len(valueSource) < 2 || !strings.HasPrefix(valueSource, `"`) || !strings.HasSuffix(valueSource, `"`) {
			return apiResponseCookieSpec{}, fmt.Errorf("cookie value has mismatched quotes")
		}
		quoted = true
		valueSource = valueSource[1 : len(valueSource)-1]
	}

	placeholder := "hyperbricks-cookie-value"
	if quoted {
		placeholder = `"` + placeholder + `"`
	}
	parseLine := name + "=" + placeholder
	if strings.TrimSpace(attributes) != "" {
		parseLine += "; " + attributes
	}
	cookie, err := http.ParseSetCookie(parseLine)
	if err != nil {
		return apiResponseCookieSpec{}, fmt.Errorf("invalid Set-Cookie syntax: %w", err)
	}
	if len(cookie.Unparsed) > 0 {
		return apiResponseCookieSpec{}, fmt.Errorf("unsupported or malformed cookie attributes: %s", strings.Join(cookie.Unparsed, ", "))
	}

	valueTemplate, fields, dynamic, err := compileAPIResponseCookieValue(valueSource)
	if err != nil {
		return apiResponseCookieSpec{}, err
	}
	if valueSource == "" && cookie.MaxAge >= 0 {
		return apiResponseCookieSpec{}, fmt.Errorf("empty value requires an explicit Max-Age=0 deletion")
	}
	return apiResponseCookieSpec{
		cookie:           *cookie,
		valueTemplate:    valueTemplate,
		valueFields:      fields,
		dynamicValue:     dynamic,
		allowsEmptyValue: cookie.MaxAge < 0,
	}, nil
}

func rejectAPIResponseCookieControlBytes(raw string) error {
	for index := 0; index < len(raw); index++ {
		if raw[index] < 0x20 || raw[index] == 0x7f {
			return fmt.Errorf("cookie string contains control byte 0x%02x at byte %d", raw[index], index)
		}
	}
	return nil
}

func validateLegacyAPIResponseCookieAttributes(attributes string) error {
	seen := make(map[string]bool)
	for _, part := range strings.Split(attributes, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, hasValue := strings.Cut(part, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		if seen[name] {
			return fmt.Errorf("duplicate cookie attribute %q", name)
		}
		seen[name] = true

		switch name {
		case "secure", "httponly", "partitioned":
			if hasValue {
				return fmt.Errorf("cookie attribute %q must not have a value", name)
			}
		case "path", "domain", "expires", "max-age", "samesite":
			if !hasValue || strings.TrimSpace(value) == "" {
				return fmt.Errorf("cookie attribute %q requires a value", name)
			}
			if name == "samesite" {
				switch strings.ToLower(strings.TrimSpace(value)) {
				case "lax", "strict", "none":
				default:
					return fmt.Errorf("unsupported SameSite value %q", value)
				}
			}
		case "":
			return fmt.Errorf("cookie attribute name must not be empty")
		default:
			return fmt.Errorf("unsupported cookie attribute %q", name)
		}
	}
	return nil
}

func parseStructuredAPIResponseCookie(raw map[string]interface{}) (apiResponseCookieSpec, error) {
	for key := range raw {
		if _, allowed := apiResponseCookieFields[key]; !allowed {
			return apiResponseCookieSpec{}, fmt.Errorf("unsupported structured cookie field %q", key)
		}
	}

	name, err := requiredAPIResponseCookieString(raw, "name")
	if err != nil {
		return apiResponseCookieSpec{}, err
	}
	if containsTemplateSyntax(name) {
		return apiResponseCookieSpec{}, fmt.Errorf("cookie name must not contain template expressions")
	}
	valueSource, err := requiredAPIResponseCookieString(raw, "value")
	if err != nil {
		return apiResponseCookieSpec{}, err
	}

	cookie := http.Cookie{Name: name}
	for _, field := range []string{"path", "domain"} {
		value, exists, err := optionalAPIResponseCookieString(raw, field)
		if err != nil {
			return apiResponseCookieSpec{}, err
		}
		if !exists {
			continue
		}
		if value == "" {
			return apiResponseCookieSpec{}, fmt.Errorf("structured cookie field %q must not be empty", field)
		}
		if containsTemplateSyntax(value) {
			return apiResponseCookieSpec{}, fmt.Errorf("cookie attribute %q must not contain template expressions", field)
		}
		if field == "path" {
			cookie.Path = value
		} else {
			cookie.Domain = value
		}
	}

	for _, field := range []string{"http_only", "secure"} {
		value, exists, err := optionalAPIResponseCookieBool(raw, field)
		if err != nil {
			return apiResponseCookieSpec{}, err
		}
		if !exists {
			continue
		}
		if field == "http_only" {
			cookie.HttpOnly = value
		} else {
			cookie.Secure = value
		}
	}

	if sameSite, exists, err := optionalAPIResponseCookieString(raw, "same_site"); err != nil {
		return apiResponseCookieSpec{}, err
	} else if exists {
		if containsTemplateSyntax(sameSite) {
			return apiResponseCookieSpec{}, fmt.Errorf("cookie attribute %q must not contain template expressions", "same_site")
		}
		switch strings.ToLower(strings.TrimSpace(sameSite)) {
		case "lax":
			cookie.SameSite = http.SameSiteLaxMode
		case "strict":
			cookie.SameSite = http.SameSiteStrictMode
		case "none":
			cookie.SameSite = http.SameSiteNoneMode
		default:
			return apiResponseCookieSpec{}, fmt.Errorf("unsupported SameSite value %q", sameSite)
		}
	}

	allowsEmptyValue := false
	if rawMaxAge, exists := raw["max_age"]; exists {
		maxAge, ok := rawMaxAge.(int)
		if !ok {
			return apiResponseCookieSpec{}, fmt.Errorf("structured cookie field %q must be an integer, got %T", "max_age", rawMaxAge)
		}
		if maxAge < 0 {
			return apiResponseCookieSpec{}, fmt.Errorf("structured cookie field %q must be zero or greater", "max_age")
		}
		if maxAge == 0 {
			cookie.MaxAge = -1
			allowsEmptyValue = true
		} else {
			cookie.MaxAge = maxAge
		}
	}

	if expires, exists, err := optionalAPIResponseCookieString(raw, "expires"); err != nil {
		return apiResponseCookieSpec{}, err
	} else if exists {
		if containsTemplateSyntax(expires) {
			return apiResponseCookieSpec{}, fmt.Errorf("cookie attribute %q must not contain template expressions", "expires")
		}
		parsed, err := http.ParseTime(expires)
		if err != nil {
			return apiResponseCookieSpec{}, fmt.Errorf("invalid cookie expires value %q: %w", expires, err)
		}
		cookie.Expires = parsed
	}

	valueTemplate, fields, dynamic, err := compileAPIResponseCookieValue(valueSource)
	if err != nil {
		return apiResponseCookieSpec{}, err
	}
	if valueSource == "" && !allowsEmptyValue {
		return apiResponseCookieSpec{}, fmt.Errorf("empty value requires max_age: 0")
	}
	return apiResponseCookieSpec{
		cookie:           cookie,
		valueTemplate:    valueTemplate,
		valueFields:      fields,
		dynamicValue:     dynamic,
		allowsEmptyValue: allowsEmptyValue,
	}, nil
}

func requiredAPIResponseCookieString(raw map[string]interface{}, field string) (string, error) {
	value, exists := raw[field]
	if !exists {
		return "", fmt.Errorf("structured cookie field %q is required", field)
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("structured cookie field %q must be a string, got %T", field, value)
	}
	if field == "name" && result == "" {
		return "", fmt.Errorf("structured cookie field %q must not be empty", field)
	}
	return result, nil
}

func optionalAPIResponseCookieString(raw map[string]interface{}, field string) (string, bool, error) {
	value, exists := raw[field]
	if !exists {
		return "", false, nil
	}
	result, ok := value.(string)
	if !ok {
		return "", true, fmt.Errorf("structured cookie field %q must be a string, got %T", field, value)
	}
	return result, true, nil
}

func optionalAPIResponseCookieBool(raw map[string]interface{}, field string) (bool, bool, error) {
	value, exists := raw[field]
	if !exists {
		return false, false, nil
	}
	result, ok := value.(bool)
	if !ok {
		return false, true, fmt.Errorf("structured cookie field %q must be a boolean, got %T", field, value)
	}
	return result, true, nil
}

func compileAPIResponseCookieValue(source string) (*template.Template, [][]string, bool, error) {
	if strings.Contains(source, "{{/*") || strings.Contains(source, "{{- /*") {
		return nil, nil, false, fmt.Errorf("cookie value template comments are not supported")
	}
	tmpl, err := template.New("api-response-cookie-value").Option("missingkey=error").Parse(source)
	if err != nil {
		return nil, nil, false, fmt.Errorf("invalid cookie value template: %w", err)
	}
	if len(tmpl.Templates()) != 1 {
		return nil, nil, false, fmt.Errorf("cookie value template definitions are not supported")
	}

	var fields [][]string
	for _, node := range tmpl.Tree.Root.Nodes {
		switch typed := node.(type) {
		case *parse.TextNode:
			continue
		case *parse.ActionNode:
			if len(typed.Pipe.Decl) != 0 || typed.Pipe.IsAssign || len(typed.Pipe.Cmds) != 1 || len(typed.Pipe.Cmds[0].Args) != 1 {
				return nil, nil, false, fmt.Errorf("cookie value templates support only direct field interpolation")
			}
			field, ok := typed.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			if !ok || len(field.Ident) == 0 {
				return nil, nil, false, fmt.Errorf("cookie value templates support only direct field interpolation")
			}
			path := append([]string(nil), field.Ident...)
			fields = append(fields, path)
		default:
			return nil, nil, false, fmt.Errorf("cookie value template construct %T is not supported", node)
		}
	}
	return tmpl, fields, len(fields) > 0, nil
}

func apiResponseCookieTemplateContext(data interface{}, status int, values map[string]interface{}) (map[string]interface{}, error) {
	context := make(map[string]interface{}, len(values)+2)
	for key, value := range values {
		if key == "Data" || key == "Status" {
			return nil, fmt.Errorf("cookie template value key %q is reserved", key)
		}
		context[key] = value
	}
	context["Data"] = data
	context["Status"] = status
	return context, nil
}

func renderAPIResponseCookieValue(spec *apiResponseCookieSpec, context map[string]interface{}) (string, error) {
	for _, field := range spec.valueFields {
		value, err := resolveAPIResponseCookieTemplateField(context, field)
		if err != nil {
			return "", err
		}
		valueType := reflect.TypeOf(value)
		if valueType == nil || valueType.Kind() != reflect.String {
			return "", fmt.Errorf("template field .%s must resolve to a string, got %T", strings.Join(field, "."), value)
		}
		if reflect.ValueOf(value).String() == "" {
			return "", fmt.Errorf("template field .%s must resolve to a nonempty string", strings.Join(field, "."))
		}
	}

	var output bytes.Buffer
	if err := spec.valueTemplate.Execute(&output, context); err != nil {
		return "", fmt.Errorf("failed to render cookie value: %w", err)
	}
	return output.String(), nil
}

func resolveAPIResponseCookieTemplateField(root interface{}, path []string) (interface{}, error) {
	current := reflect.ValueOf(root)
	for _, field := range path {
		for current.IsValid() && (current.Kind() == reflect.Interface || current.Kind() == reflect.Pointer) {
			if current.IsNil() {
				return nil, fmt.Errorf("template field .%s resolved through a null value", strings.Join(path, "."))
			}
			current = current.Elem()
		}
		if !current.IsValid() {
			return nil, fmt.Errorf("template field .%s is missing or null", strings.Join(path, "."))
		}

		switch current.Kind() {
		case reflect.Map:
			if current.Type().Key().Kind() != reflect.String {
				return nil, fmt.Errorf("template field .%s cannot be resolved from map key type %s", strings.Join(path, "."), current.Type().Key())
			}
			key := reflect.ValueOf(field)
			if !key.Type().AssignableTo(current.Type().Key()) {
				key = key.Convert(current.Type().Key())
			}
			current = current.MapIndex(key)
			if !current.IsValid() {
				return nil, fmt.Errorf("template field .%s is missing", strings.Join(path, "."))
			}
		case reflect.Struct:
			current = current.FieldByName(field)
			if !current.IsValid() || !current.CanInterface() {
				return nil, fmt.Errorf("template field .%s is missing", strings.Join(path, "."))
			}
		default:
			return nil, fmt.Errorf("template field .%s cannot be resolved through %s", strings.Join(path, "."), current.Kind())
		}
	}

	for current.IsValid() && current.Kind() == reflect.Interface {
		if current.IsNil() {
			return nil, fmt.Errorf("template field .%s is null", strings.Join(path, "."))
		}
		current = current.Elem()
	}
	if !current.IsValid() || !current.CanInterface() {
		return nil, fmt.Errorf("template field .%s is missing or null", strings.Join(path, "."))
	}
	return current.Interface(), nil
}

func validateAPIResponseCookie(cookie *http.Cookie, incoming *http.Request) error {
	if len(cookie.Unparsed) > 0 {
		return fmt.Errorf("unsupported or malformed cookie attributes: %s", strings.Join(cookie.Unparsed, ", "))
	}
	if err := cookie.Valid(); err != nil {
		return err
	}
	if cookie.Path != "" && !strings.HasPrefix(cookie.Path, "/") {
		return fmt.Errorf("cookie Path must start with /")
	}
	if cookie.SameSite == http.SameSiteNoneMode && !cookie.Secure {
		return fmt.Errorf("SameSite=None requires Secure")
	}
	if strings.HasPrefix(cookie.Name, "__Secure-") && !cookie.Secure {
		return fmt.Errorf("__Secure- cookies require Secure")
	}
	if strings.HasPrefix(cookie.Name, "__Host-") {
		if !cookie.Secure || cookie.Path != "/" || cookie.Domain != "" {
			return fmt.Errorf("__Host- cookies require Secure, Path=/, and no Domain")
		}
	}
	if strings.HasPrefix(cookie.Name, "__Http-") && (!cookie.Secure || !cookie.HttpOnly) {
		return fmt.Errorf("__Http- cookies require Secure and HttpOnly")
	}
	if strings.HasPrefix(cookie.Name, "__Host-Http-") {
		if !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.Domain != "" {
			return fmt.Errorf("__Host-Http- cookies require Secure, HttpOnly, Path=/, and no Domain")
		}
	}
	return validateAPIResponseCookieDomain(cookie.Domain, incoming)
}

func validateAPIResponseCookieDomain(configuredDomain string, incoming *http.Request) error {
	if configuredDomain == "" {
		return nil
	}
	domain := strings.ToLower(strings.TrimPrefix(configuredDomain, "."))
	if domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid cookie Domain %q", configuredDomain)
	}
	publicSuffix, _ := publicsuffix.PublicSuffix(domain)
	if net.ParseIP(domain) == nil && publicSuffix == domain {
		return fmt.Errorf("cookie Domain %q is a public suffix", configuredDomain)
	}
	if incoming == nil {
		return fmt.Errorf("cookie Domain requires an incoming request for domain matching")
	}
	host := incomingRequestHostname(incoming)
	if host == "" {
		return fmt.Errorf("cookie Domain requires an incoming request host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if domain != strings.ToLower(host) {
			return fmt.Errorf("cookie Domain %q does not match request host %q", configuredDomain, host)
		}
		return nil
	}
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		return fmt.Errorf("cookie Domain %q does not match request host %q", configuredDomain, host)
	}
	return nil
}

func incomingRequestHostname(request *http.Request) string {
	if request == nil {
		return ""
	}
	host := ""
	if request.URL != nil {
		host = request.URL.Hostname()
	}
	if host == "" {
		host = request.Host
		if splitHost, _, err := net.SplitHostPort(host); err == nil {
			host = splitHost
		} else {
			host = strings.Trim(host, "[]")
		}
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

func containsTemplateSyntax(value string) bool {
	return strings.Contains(value, "{{") || strings.Contains(value, "}}")
}
