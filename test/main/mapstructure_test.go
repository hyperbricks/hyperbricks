package main

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/parser"
	"github.com/hyperbricks/hyperbricks/internal/render"
	"github.com/hyperbricks/hyperbricks/internal/renderer"

	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/internal/typefactory"
)

// Example Renderer implementation
type MyRenderer struct {
	renderManager render.RenderManager
}

// Validate ensures that the endpoint has valid data.
func (endpoint *MyRenderer) Validate() []error {
	var warnings []error
	return warnings
}

// Ensure PageRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*MyRenderer)(nil)

func MyRendererConfigGetName() string {
	return "<MYRENDERER>"
}

func (r *MyRenderer) Types() []string {
	return []string{
		MyRendererConfigGetName(),
	}
}

func (r *MyRenderer) Render(instance interface{}) (string, []error) {

	// Check if renderManager is of the type render.RenderManager
	if reflect.TypeOf(&r.renderManager) == reflect.TypeOf(render.RenderManager{}) {
		fmt.Print("RENDER MANAGER!!")
	}

	config, ok := instance.(TestCompositeConfig)
	if !ok {
		return "", []error{
			shared.CompositeError{
				Err: fmt.Errorf("invalid type for MyRenderer").Error(),
			},
		}
	}

	return "Rendered content: " + config.Href, nil
}

// Basic config for ComponentRenderers
type TestComponentRendererConfig struct {
	shared.Component `mapstructure:",squash"`
	Value            string `mapstructure:"value"`
}

type TestCompositeConfig struct {
	shared.Composite `mapstructure:",squash"`
	Href             string `mapstructure:"title"`
}

// Basic config for ComponentRenderers
type LinkRendererConfig struct {
	shared.Component `mapstructure:",squash"`
	Href             string                 `mapstructure:"href" validate:"required" description:"URL of the link" example:"{!{link-href.hyperbricks}}"`
	Text             string                 `mapstructure:"text" validate:"required" description:"Text to display for the link" example:"{!{link-text.hyperbricks}}"`
	Target           string                 `mapstructure:"target" description:"Target attribute for the link (_blank, _self, etc.)" example:"{!{link-target.hyperbricks}}"`
	Classes          []string               `mapstructure:"classes" description:"Optional CSS classes for the link" example:"{!{link-classes.hyperbricks}}"`
	Wrap             string                 `mapstructure:"enclose" description:"The enclosing HTML element for the header divided by |" example:"{!{link-enclose.hyperbricks}}"`
	ExtraAttributes  map[string]interface{} `mapstructure:"attributes"  description:"Extra attributes like id, data-role, data-action" example:"{!{link-attributes.hyperbricks}}"`
}

func Test_Struct_Link_Validation(t *testing.T) {

	apiConfig := &component.APIConfig{}

	// returns nil or ValidationErrors ( []FieldError )
	errors := shared.Validate(apiConfig)

	for _, err := range errors {
		e := err.(shared.ComponentError)
		fmt.Println(e.Err)
	}

}

// User contains user information
type User struct {
	shared.Component `mapstructure:",squash"`
	FirstName        string     `validate:"required"`
	LastName         string     `validate:"required"`
	Age              uint8      `validate:"gte=0,lte=130"`
	Email            string     `validate:"required,email"`
	Gender           string     `validate:"oneof=male female prefer_not_to"`
	FavouriteColor   string     `validate:"iscolor"`                // alias for 'hexcolor|rgb|rgba|hsl|hsla'
	Addresses        []*Address `validate:"required,dive,required"` // a person can have a home and cottage...
}

// Address houses a users address information
type Address struct {
	Street string `validate:"required"`
	City   string `validate:"required"`
	Planet string `validate:"required"`
	Phone  string `validate:"required"`
}

func Test_Struct_User_Validation(t *testing.T) {

	address := &Address{
		Street: "Eavesdown Docks",
		Planet: "Persphone",
		Phone:  "none",
	}

	user := &User{
		FirstName:      "Badger",
		LastName:       "Smith",
		Age:            135,
		Gender:         "male",
		Email:          "Badger.Smith@gmail.com",
		FavouriteColor: "#000-",
		Addresses:      []*Address{address},
	}

	// returns nil or ValidationErrors ( []FieldError )
	errors := shared.Validate(user)
	if errors != nil {
		for _, err := range errors {
			e := err.(shared.ComponentError)
			fmt.Println(e.Err)
		}
	}

}

func Test_AnotherMapstructureEmbedding(t *testing.T) {
	testTestCompositeConfig := TestCompositeConfig{
		Composite: shared.Composite{
			Meta: shared.Meta{
				ConfigType: "TEST",
			},
		},
	}

	// Check direct field access
	if testTestCompositeConfig.ConfigType != "TEST" {
		t.Errorf("Expected ConfigType to be TEST")
	}

	// Check embedded struct access
	if testTestCompositeConfig.Composite.ConfigType != "TEST" {
		t.Errorf("Expected Composite.ConfigType to be TEST")
	}
}

func Test_MapstructureEmbedding(t *testing.T) {
	testTestCompositeConfig := TestCompositeConfig{}
	testTestCompositeConfig.ConfigType = "TEST"
	if testTestCompositeConfig.ConfigType != "TEST" {
		t.Errorf("Wtf, there has to be TEST")
	}

	if testTestCompositeConfig.Composite.ConfigType != "TEST" {
		t.Errorf("Wtf, there has to be TEST")
	}
}

func Test_IfRendermanagerRendersAComponentRenderer(t *testing.T) {
	input := `
   
	compositetest = <MYRENDERER>
	compositetest {
		href = http://www.example.com
	}
    `
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()
	// Create a RenderManager for MyError type
	rm := render.NewRenderManager()
	rm.RegisterComponent(MyRendererConfigGetName(), &MyRenderer{}, reflect.TypeOf(TestCompositeConfig{}))

	// Parse the input
	parsedConfig := parser.ParseHyperScript(input)
	c := parser.KnownTypes

	result, errr := rm.Render(MyRendererConfigGetName(), parsedConfig["compositetest"].(map[string]interface{}))
	if errr != nil {
		t.Errorf("Rendering problem %v", result)
		log.Printf("%v", c)
	}

	// Create a TypeRequest for the TypeFactory
	request := typefactory.TypeRequest{
		TypeName: MyRendererConfigGetName(),
		Data:     parsedConfig["compositetest"].(map[string]interface{}),
	}

	// Create the instance using TypeFactory
	response, err := rm.MakeInstance(request)
	if err != nil {
		t.Errorf("Error creating instance: %v", err)
		return
	}

	config, ok := response.Instance.(TestCompositeConfig)
	if !ok {
		t.Errorf("Error instance not a TestCompositeConfig type: %v", err)
	}

	configType := config.Composite.Meta.ConfigType
	config.Composite.Meta.ConfigType = "SET_RENDER"

	if configType != MyRendererConfigGetName() {
		t.Errorf("Type not correctly set")
	}

	if config.Composite.Meta.ConfigType != "SET_RENDER" {
		t.Errorf("Type not correctly set")
	}

}

func Test_BasicRenderChain(t *testing.T) {
	input := `
   
	compositetest = <TREE>
	compositetest {
		
		10 = <HTML>
		10.value = <a href="LINK 10">somelink</a>
		25 = stom

		40 = <HTML>
		40.value = <a href="LINK 30">somelink</a><a href="LINK 40">somelink</a>
	}
    `
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()

	// Create a RenderManager for MyError type
	rm := render.NewRenderManager()
	// Mock template provider
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"api_test_template": `{{ (index .quotes 0).author }}:{{ (index .quotes 0).quote }}`,
		}
		content, exists := templates[templateName]
		return content, exists
	}
	// TEMPLATE ....
	pageRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), pageRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	// Parse the input
	parsedConfig := parser.ParseHyperScript(input)

	// logic...
	// if parsedConfig["compositetest"] type == "RENDER" then ....
	result, errr := rm.Render(composite.TreeRendererConfigGetName(), parsedConfig["compositetest"].(map[string]interface{}))
	// this could in theory used for page too...
	// page....
	// 		HEAD
	//      TEMPLATE -> RENDER ITEMS AS VALUES?

	if errr != nil {
		fmt.Println("EXPECT ERRORS:")
		for _, err := range errr {
			e := err.(shared.ComponentError)
			fmt.Println(e.Err)
		}
	}

	if len(errr) == 0 {
		t.Errorf("expected errors")
	}

	expect := `<a href="LINK 10">somelink</a><!-- begin raw value -->stom<!-- end raw value --><a href="LINK 30">somelink</a><a href="LINK 40">somelink</a>`

	fmt.Printf("result: %s\n\n\n", result)
	if _normalizeString(result) != _normalizeString(expect) {
		t.Errorf("expected %s got %s", expect, result)
	}

}

func Test_BasicHyperMediaRenderChain(t *testing.T) {
	input := `
   
	hypermedia = <HYPERMEDIA>
	hypermedia {
		css = [a,b,c,d]
		head {
			10 = <JAVASCRIPT>
			10.inline = <<[console.log("Hello World")]>>
		}

		title = test title

		10 = <HTML>
		10.value = <a href="#LINK_10">LINK_10</a>

		20 = <TREE>
		20 = {
			10 = <HTML>
			10.value = <a href="#LINK_20_10">LINK_20_10</a>
		}
		
		25 = no_type

		50 = UNKNOWN
		50.href = LINK 30

		30 = <HTML>
		30.value = <a href="#LINK_30">LINK_30</a>

		40 = <HTML>
		40.value = <a href="#LINK_40">LINK_40</a>
	}
    `
	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()
	// Create a RenderManager for MyError type
	rm := render.NewRenderManager()
	// Mock template provider
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"api_test_template": `{{ (index .quotes 0).author }}:{{ (index .quotes 0).quote }}`,
		}
		content, exists := templates[templateName]
		return content, exists
	}
	// TEMPLATE ....
	pageRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), pageRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))

	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))

	// Parse the input
	parsedConfig := parser.ParseHyperScript(input)
	// logic...
	// if parsedConfig["compositetest"] type == "RENDER" then ....
	result, errr := rm.Render(composite.HyperMediaConfigGetName(), parsedConfig["hypermedia"].(map[string]interface{}))
	// this could in theory used for page too...
	// page....
	// 		HEAD
	//      TEMPLATE -> RENDER ITEMS AS VALUES?

	if errr != nil {
		fmt.Println("EXPECT ERRORS:")
		for _, err := range errr {
			e := err.(shared.ComponentError)
			fmt.Println(e.Err)
		}
	}

	if len(errr) == 0 {
		t.Errorf("expected errors")
	}

	expect := `<!DOCTYPE html><html><head><script>
        console.log("Hello World")
        </script><meta name="generator" content="hyperbricks cms"><title>test title</title>
        </head><body><a href="#LINK_10">LINK_10</a><a href="#LINK_20_10">LINK_20_10</a><!-- begin raw value -->no_type<!-- end raw value --><a href="#LINK_30">LINK_30</a><a href="#LINK_40">LINK_40</a></body></html>`

	fmt.Printf("result: %s\n\n\n", _normalizeString(result))
	if _normalizeString(result) != _normalizeString(expect) {
		t.Errorf("expected %s got %s", expect, result)
	}

}

// ignores added content like 10, 20 etc because it has a template added. So document structure from template, with head value for passing head parts...
func Test_BasicPageWithTemplateRenderChain(t *testing.T) {
	input := `
   
	hypermedia = <HYPERMEDIA>
	hypermedia {
		template {
			 template = my_template
			 values {
			 	a = AAAAA
				b = BBBBB
			 }
		}
		css = [a,b,c,d]
		head {
			10 = AQUACADABRA
		
		}

		title = test title

		10 = <HTML>
		10.value = <a href="#LINK_10">LINK_10</a>

		20 = <TREE>
		20 = {
			10 = <HTML>
			10.value = <a href="#LINK_20_10">LINK_20_10</a>
		}
		
		25 = no_type

		50 = UNKNOWN
		50.href = LINK 30

		30 = <HTML>
		30.value = <a href="#LINK_30">LINK_30</a>

		40 = <HTML>
		40.value = <a href="#LINK_40">LINK_40</a>
	}
    `

	shared.Init_configuration()
	shared.GetHyperBricksConfiguration()
	// Create a RenderManager for MyError type
	rm := render.NewRenderManager()
	// Mock template provider
	templateProvider := func(templateName string) (string, bool) {
		templates := map[string]string{
			"api_test_template": `{{ (index .quotes 0).author }}:{{ (index .quotes 0).quote }}`,
			"my_template":       `<!DOCTYPE html><html>{{head}}<body><div id="val_b">{{.b}} {{b}}</div><div id="val_a">{{a}}</div><div id="d">{{d}}</div></body></html>`,
		}
		content, exists := templates[templateName]
		return content, exists
	}
	// TEMPLATE ....
	pageRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.HyperMediaConfigGetName(), pageRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))

	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}

	rm.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))

	// TEMPLATE ....
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager:    rm,
			TemplateProvider: templateProvider,
		},
	}

	rm.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))

	// API ....
	apiRenderer := &component.APIRenderer{
		ComponentRenderer: renderer.ComponentRenderer{
			TemplateProvider: templateProvider,
		},
	}

	headRenderer := &composite.HeadRenderer{
		CompositeRenderer: renderer.CompositeRenderer{
			RenderManager: rm,
		},
	}
	rm.RegisterComponent(composite.HeadConfigGetName(), headRenderer, reflect.TypeOf(composite.HeadConfig{}))

	// COMPONENTS
	rm.RegisterComponent(component.APIConfigGetName(), apiRenderer, reflect.TypeOf(component.APIConfig{}))

	rm.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))

	rm.RegisterComponent(component.CssConfigGetName(), &component.CssRenderer{}, reflect.TypeOf(component.CssConfig{}))

	rm.RegisterComponent(component.StyleConfigGetName(), &component.StyleRenderer{}, reflect.TypeOf(component.StyleConfig{}))
	rm.RegisterComponent(component.JavaScriptConfigGetName(), &component.JavaScriptRenderer{}, reflect.TypeOf(component.JavaScriptConfig{}))

	// Parse the input
	parsedConfig := parser.ParseHyperScript(input)
	// logic...
	// if parsedConfig["compositetest"] type == "RENDER" then ....
	result, errr := rm.Render(composite.HyperMediaConfigGetName(), parsedConfig["hypermedia"].(map[string]interface{}))
	// this could in theory used for page too...
	// page....
	// 		HEAD
	//      TEMPLATE -> RENDER ITEMS AS VALUES?

	if errr != nil {
		fmt.Println("EXPECT ERRORS:")
		for _, err := range errr {
			e := err.(shared.ComponentError)
			fmt.Println(e.Err)
		}
	}

	if len(errr) == 0 {
		t.Errorf("expected errors")
	}

	expect := `<!DOCTYPE html><html><head><!-- begin raw value -->AQUACADABRA<!-- end raw value --><meta name="generator" content="hyperbricks cms"><title>test title</title>
        </head><body><div id="val_b">BBBBB BBBBB</div><div id="val_a">AAAAA</div><div id="d"></div></body></html>`

	fmt.Printf("result: %s\n\n\n", _normalizeString(result))
	if _normalizeString(result) != _normalizeString(expect) {
		t.Errorf("expected %s got %s", expect, result)
	}

}

// normalizeString trims and removes excess whitespace for comparison purposes.
func _normalizeString(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
