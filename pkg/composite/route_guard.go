package composite

// RouteGuardConfig defines an optional pre-render route guard for route-owning
// composites.
type RouteGuardConfig struct {
	Enabled           bool                       `mapstructure:"enabled" description:"Enable route guarding before the route is rendered"`
	Auth              RouteGuardAuthConfig       `mapstructure:"auth" description:"Authentication token extraction settings"`
	Require           RouteGuardRequireConfig    `mapstructure:"require" description:"Additional route requirements"`
	Authorize         *RouteGuardAuthorizeConfig `mapstructure:"authorize" description:"Optional upstream authorization check performed before rendering"`
	OnUnauthenticated RouteGuardActionConfig     `mapstructure:"on_unauthenticated" description:"Response behavior when authentication is missing or invalid"`
	OnForbidden       RouteGuardActionConfig     `mapstructure:"on_forbidden" description:"Response behavior when authorization fails"`
}

type RouteGuardAuthConfig struct {
	Cookie string `mapstructure:"cookie" description:"Cookie name used to resolve the request token"`
	Header string `mapstructure:"header" description:"Header name used to resolve the request token, defaults to Authorization"`
	Scheme string `mapstructure:"scheme" description:"Optional header scheme, defaults to Bearer for Authorization headers"`
}

type RouteGuardRequireConfig struct {
	Authenticated bool                   `mapstructure:"authenticated" description:"Require an authenticated request before rendering"`
	Query         map[string]interface{} `mapstructure:"query" description:"Required query keys, set each key to true to enforce presence"`
}

type RouteGuardAuthorizeConfig struct {
	Endpoint string            `mapstructure:"endpoint" description:"Optional authorization endpoint called before rendering"`
	Method   string            `mapstructure:"method" description:"HTTP method for the authorization endpoint"`
	Headers  map[string]string `mapstructure:"headers" description:"Optional headers sent to the authorization endpoint"`
	Body     string            `mapstructure:"body" description:"Optional request body with $key placeholder interpolation from the incoming request"`
}

type RouteGuardActionConfig struct {
	Default  HTTPResponseConfig          `mapstructure:"default" description:"Default browser response when this guard denies access"`
	Variants []RouteGuardResponseVariant `mapstructure:"variants" description:"Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely"`
}

type RouteGuardResponseVariant struct {
	When     RouteGuardRequestMatch `mapstructure:"when"`
	Response HTTPResponseConfig     `mapstructure:"response"`
}

type RouteGuardRequestMatch struct {
	RequestHeaders map[string]string `mapstructure:"request_headers"`
}

// Backward-compatible aliases for the original hypermedia-specific naming.
type HyperMediaGuardConfig = RouteGuardConfig
type HyperMediaGuardAuthConfig = RouteGuardAuthConfig
type HyperMediaGuardRequireConfig = RouteGuardRequireConfig
type HyperMediaGuardAuthorizeConfig = RouteGuardAuthorizeConfig
type HyperMediaGuardActionConfig = RouteGuardActionConfig
