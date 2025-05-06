package api

import (
	"context"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/customeros/leads/api/graphql/generated"
	"github.com/customeros/leads/api/graphql/resolver"
	"github.com/customeros/leads/api/handlers"
	"github.com/customeros/leads/api/middleware"
	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/utils"
	"github.com/customeros/leads/services"
)

// RegisterRoutes sets up all API endpoints
func RegisterRoutes(ctx context.Context, r *gin.Engine, services *services.Services, config *config.AppConfig) *handlers.APIHandlers {
	if services == nil {
		panic("Services cannot be nil")
	}
	if config == nil {
		panic("Config cannot be nil")
	}

	// Add recovery middlewares
	r.Use(gin.Recovery()) // Gin's built-in recovery

	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"}
	corsConfig.AllowHeaders = []string{
		"Origin",
		"Content-Type",
		"Accept",
		"Authorization",
		"X-Requested-With",
		"X-Tenant-ID",
		"X-User-ID",
	}
	corsConfig.ExposeHeaders = []string{"Content-Length"}
	corsConfig.AllowCredentials = true
	r.Use(cors.New(corsConfig))

	// setup handlers
	apiHandlers := handlers.InitHandlers(services)

	// Health check and status endpoints (no custom context needed)
	r.GET("/health", handlers.HealthCheck)

	// Rest API
	api := r.Group("/v1")
	{
		// Domain endpoints
		events := api.Group("/events")
		events.Use(middleware.CustomContextMiddleware()) // Add custom context
		events.Use(middleware.TracingMiddleware(ctx))    // Add tracing with parent context
		{
			events.POST("", apiHandlers.WebEvents.Handle())
		}
	}

	// GraphQL API
	graphqlHandler, playgroundHandler := SetupGraphQLServer(services)

	graphql := r.Group("/")
	{
		graphql.GET("/", playgroundHandler) // playground
	}

	query := r.Group("/query")
	query.Use(middleware.TenantValidationMiddleware()) // Tenant header validation
	query.Use(middleware.UserIdMiddleware())           // UserId header parsing
	query.Use(middleware.CustomContextMiddleware())    // Add custom context
	query.Use(middleware.TracingMiddleware(ctx))       // Add tracing with parent context
	query.POST("", graphqlHandler)                     // query
	query.OPTIONS("", func(c *gin.Context) {           // Handle OPTIONS requests
		c.Status(200)
	})

	return apiHandlers
}

// SetupGraphQLServer configures and returns the GraphQL server and playground handlers
func SetupGraphQLServer(services *services.Services) (graphqlHandler, playgroundHandler gin.HandlerFunc) {
	// Create the resolver with dependencies
	resolver := resolver.NewResolver(services)

	// Create a new schema with your resolvers
	schema := generated.NewExecutableSchema(generated.Config{
		Resolvers:  resolver,
		Directives: generated.DirectiveRoot{},
		Complexity: generated.ComplexityRoot{},
	})

	// Create the GraphQL server with custom options
	srv := handler.New(schema)

	// Configure server options
	srv.AddTransport(transport.POST{})          // Support POST requests
	srv.AddTransport(transport.GET{})           // Support GET requests
	srv.AddTransport(transport.MultipartForm{}) // Support multipart form
	srv.AddTransport(transport.Options{})       // Support OPTIONS requests
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	// Add extensions
	srv.Use(extension.Introspection{}) // Enable introspection
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	// Create playground handler
	playground := playground.Handler("GraphQL", "/query")

	// Return handlers wrapped for Gin with our custom middleware
	return func(c *gin.Context) {
		// Explicitly add the Gin context to the request context
		ginCtx := middleware.GinContextToContextMiddleware()
		ginCtx(c)

		// Add custom middleware to extract tenant from Gin context
		c.Request = c.Request.WithContext(utils.WithTenantContext(c.Request.Context(), c.GetString("Tenant")))

		// Call the GraphQL handler
		srv.ServeHTTP(c.Writer, c.Request)
	}, gin.WrapH(playground)
}
