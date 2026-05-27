package schema

import (
	"reflect"
	"sort"
	"strings"
)

type ComponentSchema struct {
	Version string       `json:"version"`
	Types   []TypeSchema `json:"types"`
}

type TypeSchema struct {
	Name         string         `json:"name"`
	Token        string         `json:"token"`
	Aliases      []string       `json:"aliases,omitempty"`
	Category     Category       `json:"category"`
	ChildModel   ChildModel     `json:"child_model"`
	ConfigStruct string         `json:"config_struct"`
	Description  string         `json:"description,omitempty"`
	FormGroups   []FormGroup    `json:"-"`
	Authoring    *TypeAuthoring `json:"authoring,omitempty"`
	Fields       []Field        `json:"fields"`
}

type Field struct {
	Path         string          `json:"path"`
	Key          string          `json:"key"`
	GoName       string          `json:"go_name"`
	GoType       string          `json:"go_type"`
	Kind         string          `json:"kind"`
	JSONName     string          `json:"json_name,omitempty"`
	Group        string          `json:"-"`
	Description  string          `json:"description,omitempty"`
	Example      string          `json:"example,omitempty"`
	Validate     string          `json:"validate,omitempty"`
	Required     bool            `json:"required,omitempty"`
	Header       string          `json:"header,omitempty"`
	ValueDynamic bool            `json:"value_dynamic,omitempty"`
	Authoring    *FieldAuthoring `json:"authoring,omitempty"`
}

type TypeAuthoring struct {
	Groups []FormGroup        `json:"groups,omitempty"`
	Slots  []AuthoringSlot    `json:"slots,omitempty"`
	Rules  []AuthoringRuleRef `json:"rules,omitempty"`
}

type AuthoringSlot struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Model           string   `json:"model"`
	Default         bool     `json:"default,omitempty"`
	PathKeyRequired bool     `json:"path_key_required,omitempty"`
	PlacementKinds  []string `json:"placement_kinds,omitempty"`
	Description     string   `json:"description,omitempty"`
}

type AuthoringRuleRef struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

type FieldAuthoring struct {
	Label   string        `json:"label,omitempty"`
	Control string        `json:"control,omitempty"`
	Group   string        `json:"group,omitempty"`
	Publish *PublishRules `json:"publish,omitempty"`
}

type PublishRules struct {
	OmitEmpty bool `json:"omit_empty,omitempty"`
	OmitFalse bool `json:"omit_false,omitempty"`
}

func ExtractRegistry(defs []Definition) ComponentSchema {
	types := make([]TypeSchema, 0, len(defs))
	for _, def := range defs {
		types = append(types, ExtractDefinition(def))
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i].Token < types[j].Token
	})
	return ComponentSchema{
		Version: "1",
		Types:   types,
	}
}

func ExtractDefinition(def Definition) TypeSchema {
	fields := walkType(def.ConfigType, "", nil)
	assignGroups(fields, def.FormGroups)
	assignAuthoring(fields)
	applyFieldAuthoringOverrides(fields, def.FieldAuthoringOverrides)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})

	return TypeSchema{
		Name:         def.Name,
		Token:        def.Token,
		Aliases:      append([]string(nil), def.Aliases...),
		Category:     def.Category,
		ChildModel:   def.ChildModel,
		ConfigStruct: configStructName(def.ConfigType),
		Description:  def.Description,
		FormGroups:   append([]FormGroup(nil), def.FormGroups...),
		Authoring:    inferTypeAuthoring(def),
		Fields:       fields,
	}
}

func walkType(rt reflect.Type, prefix string, fields []Field) []Field {
	rt = indirectType(rt)
	if rt.Kind() != reflect.Struct {
		return fields
	}

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}
		if sf.Tag.Get("exclude") == "true" {
			continue
		}

		tag := sf.Tag.Get("mapstructure")
		name, options := parseMapstructureTag(tag)
		if options["remain"] {
			continue
		}
		if options["squash"] {
			fields = walkType(sf.Type, prefix, fields)
			continue
		}
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "@") {
			continue
		}

		path := joinPath(prefix, name)
		ft := indirectType(sf.Type)
		if ft.Kind() == reflect.Struct && !isScalarStruct(ft) {
			fields = walkType(ft, path, fields)
			continue
		}

		validate := sf.Tag.Get("validate")
		kind := fieldKind(sf.Type)
		required := isRequired(validate)
		valueDynamic := isDynamicValueMap(path, sf.Type)
		fields = append(fields, Field{
			Path:         path,
			Key:          name,
			GoName:       sf.Name,
			GoType:       typeName(sf.Type),
			Kind:         kind,
			JSONName:     parseJSONName(sf.Tag.Get("json")),
			Description:  strings.TrimSpace(sf.Tag.Get("description")),
			Example:      strings.TrimSpace(sf.Tag.Get("example")),
			Validate:     validate,
			Required:     required,
			Header:       sf.Tag.Get("header"),
			ValueDynamic: valueDynamic,
		})
	}
	return fields
}

func assignAuthoring(fields []Field) {
	for i := range fields {
		fields[i].Authoring = inferFieldAuthoring(fields[i])
	}
}

func applyFieldAuthoringOverrides(fields []Field, overrides []FieldAuthoringOverride) {
	if len(overrides) == 0 {
		return
	}

	byPath := make(map[string]FieldAuthoringOverride, len(overrides))
	for _, override := range overrides {
		path := strings.TrimSpace(override.Path)
		if path == "" {
			continue
		}
		override.Path = path
		byPath[path] = override
	}

	for i := range fields {
		override, ok := byPath[fields[i].Path]
		if !ok {
			continue
		}
		if fields[i].Authoring == nil {
			fields[i].Authoring = &FieldAuthoring{}
		}
		if override.Label != "" {
			fields[i].Authoring.Label = override.Label
		}
		if override.Control != "" {
			fields[i].Authoring.Control = override.Control
		}
		if override.Group != "" {
			fields[i].Authoring.Group = override.Group
		}
	}
}

func inferTypeAuthoring(def Definition) *TypeAuthoring {
	slots := append([]AuthoringSlot(nil), def.AuthoringSlots...)
	if len(slots) == 0 {
		slots = inferAuthoringSlots(def.ChildModel)
	}
	authoring := TypeAuthoring{
		Groups: append([]FormGroup(nil), def.FormGroups...),
		Slots:  slots,
		Rules:  inferAuthoringRules(def),
	}
	if len(authoring.Groups) == 0 && len(authoring.Slots) == 0 && len(authoring.Rules) == 0 {
		return nil
	}
	return &authoring
}

func inferAuthoringSlots(childModel ChildModel) []AuthoringSlot {
	switch childModel {
	case ChildModelTree:
		return []AuthoringSlot{bodyAuthoringSlot(true)}
	case ChildModelHead:
		return []AuthoringSlot{headAuthoringSlot(true)}
	case ChildModelValues:
		return []AuthoringSlot{{
			Key:             "values",
			Label:           "Values",
			Model:           string(childModel),
			Default:         true,
			PathKeyRequired: true,
			PlacementKinds:  []string{"owned", "inherited"},
			Description:     "Value-mounted bricks rendered at values.<key>; normal body children are not allowed.",
		}}
	default:
		return nil
	}
}

func bodyAuthoringSlot(isDefault bool) AuthoringSlot {
	return AuthoringSlot{
		Key:             "body",
		Label:           "Body",
		Model:           string(ChildModelTree),
		Default:         isDefault,
		PathKeyRequired: false,
		PlacementKinds:  []string{"owned", "inherited"},
		Description:     "Ordered child bricks rendered below this composite.",
	}
}

func headAuthoringSlot(isDefault bool) AuthoringSlot {
	return AuthoringSlot{
		Key:             "head",
		Label:           "Head",
		Model:           string(ChildModelHead),
		Default:         isDefault,
		PathKeyRequired: false,
		PlacementKinds:  []string{"owned", "inherited"},
		Description:     "Ordered child bricks rendered as document head content.",
	}
}

func inferAuthoringRules(def Definition) []AuthoringRuleRef {
	if def.ChildModel != ChildModelValues {
		return nil
	}
	return []AuthoringRuleRef{{
		Key:         "template_value_mounts_only",
		Description: "Use value rows for scalar values, inherited value references, reusable brick references, or inline owned bricks mounted at values.<key>.",
	}}
}

func inferFieldAuthoring(field Field) *FieldAuthoring {
	authoring := FieldAuthoring{
		Label:   humanizeFieldLabel(field.Path),
		Control: inferFieldControl(field),
		Group:   field.Group,
		Publish: inferPublishRules(field.Path, field.Kind, field.Required, field.ValueDynamic),
	}
	if authoring.Label == "" && authoring.Control == "" && authoring.Group == "" && authoring.Publish == nil {
		return nil
	}
	return &authoring
}

func inferPublishRules(path string, kind string, required bool, valueDynamic bool) *PublishRules {
	rules := PublishRules{
		OmitEmpty: true,
	}

	if shouldOmitFalseOnPublish(path, kind, required, valueDynamic) {
		rules.OmitFalse = true
	}

	if !rules.OmitEmpty && !rules.OmitFalse {
		return nil
	}
	return &rules
}

func shouldOmitFalseOnPublish(path string, kind string, required bool, valueDynamic bool) bool {
	if valueDynamic {
		return false
	}
	if kind == "bool" {
		return true
	}
	if required {
		return false
	}
	key := path
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		key = key[idx+1:]
	}
	return key != "value"
}

func inferFieldControl(field Field) string {
	if field.ValueDynamic {
		return "template-values"
	}
	switch field.Kind {
	case "bool":
		return "checkbox"
	case "int", "uint", "float":
		return "number"
	case "list":
		return "list"
	case "map":
		return "key-value"
	}

	key := strings.ToLower(strings.TrimSpace(field.Key))
	path := strings.ToLower(strings.TrimSpace(field.Path))
	switch {
	case key == "template" || strings.HasSuffix(path, ".template"):
		return "template-asset"
	case key == "file" || key == "src" || key == "directory" || key == "favicon" || key == "link":
		return "asset"
	case key == "inline" || key == "body" || key == "value" || key == "enclose":
		return "textarea"
	case key == "route":
		return "route"
	case key == "method":
		return "http-method"
	default:
		return "text"
	}
}

func humanizeFieldLabel(path string) string {
	segment := strings.TrimSpace(path)
	if idx := strings.LastIndex(segment, "."); idx >= 0 {
		segment = segment[idx+1:]
	}
	segment = strings.Trim(segment, "_")
	if segment == "" {
		return ""
	}
	if label, ok := canonicalFieldLabels[strings.ToLower(segment)]; ok {
		return label
	}
	words := strings.FieldsFunc(segment, func(r rune) bool {
		return r == '_' || r == '-'
	})
	for i, word := range words {
		words[i] = humanizeWord(word)
	}
	return strings.Join(words, " ")
}

var canonicalFieldLabels = map[string]string{
	"alt":          "Alt text",
	"bodytag":      "Body tag",
	"content_type": "Content type",
	"doctype":      "Doctype",
	"htmltag":      "HTML tag",
	"jwtclaims":    "JWT claims",
	"jwtsecret":    "JWT secret",
	"nocache":      "No cache",
	"querykeys":    "Query keys",
	"queryparams":  "Query params",
	"src":          "Source",
	"trimspace":    "Trim space",
}

func humanizeWord(word string) string {
	lower := strings.ToLower(strings.TrimSpace(word))
	if lower == "" {
		return ""
	}
	switch lower {
	case "id":
		return "ID"
	case "url":
		return "URL"
	case "api":
		return "API"
	case "css":
		return "CSS"
	case "js":
		return "JS"
	case "json":
		return "JSON"
	case "jwt":
		return "JWT"
	case "html":
		return "HTML"
	case "hx":
		return "HX"
	}
	return strings.ToUpper(lower[:1]) + lower[1:]
}

func assignGroups(fields []Field, groups []FormGroup) {
	groupByField := make(map[string]string)
	for _, group := range groups {
		for _, field := range group.Fields {
			groupByField[field] = group.Key
		}
	}
	for i := range fields {
		if group, ok := groupByField[fields[i].Path]; ok {
			fields[i].Group = group
			continue
		}
		if group, ok := groupByField[fields[i].Key]; ok {
			fields[i].Group = group
		}
	}
}

func parseMapstructureTag(tag string) (string, map[string]bool) {
	options := make(map[string]bool)
	if tag == "" {
		return "", options
	}
	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part != "" {
			options[part] = true
		}
	}
	return name, options
}

func parseJSONName(tag string) string {
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "-" {
		return ""
	}
	return name
}

func joinPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func indirectType(rt reflect.Type) reflect.Type {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return rt
}

func configStructName(rt reflect.Type) string {
	rt = indirectType(rt)
	if rt.PkgPath() == "" {
		return rt.Name()
	}
	return rt.PkgPath() + "." + rt.Name()
}

func typeName(rt reflect.Type) string {
	if rt.Kind() == reflect.Ptr {
		return "*" + typeName(rt.Elem())
	}
	return rt.String()
}

func fieldKind(rt reflect.Type) string {
	rt = indirectType(rt)
	switch rt.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "list"
	case reflect.Map:
		return "map"
	case reflect.Struct:
		return "object"
	default:
		return rt.Kind().String()
	}
}

func isScalarStruct(rt reflect.Type) bool {
	return rt.PkgPath() == "time" && rt.Name() == "Time"
}

func isRequired(validate string) bool {
	for _, part := range strings.Split(validate, ",") {
		if strings.TrimSpace(part) == "required" {
			return true
		}
	}
	return false
}

func isDynamicValueMap(path string, rt reflect.Type) bool {
	rt = indirectType(rt)
	if rt.Kind() != reflect.Map {
		return false
	}
	return path == "values" || strings.HasSuffix(path, ".values")
}
